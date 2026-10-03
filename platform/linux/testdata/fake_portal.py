#!/usr/bin/env python3
"""Fake org.freedesktop.portal.Desktop exposing GlobalShortcuts, for tests.

Usage: fake_portal.py MODE   (MODE: grant | refuse | cancel-session)

Owns org.freedesktop.portal.Desktop on the session bus, then prints READY.
Every call it receives is printed to stdout as one JSON line. Commands are
read from stdin, one per line:

  activate ID          emit Activated(ID) on the last bound session
  activate-foreign ID  emit Activated(ID) on a session the client never got

Exits when stdin closes.
"""
import json
import os
import sys

from gi.repository import Gio, GLib

MODE = sys.argv[1] if len(sys.argv) > 1 else "grant"
BUS = "org.freedesktop.portal.Desktop"
PATH = "/org/freedesktop/portal/desktop"
GS = "org.freedesktop.portal.GlobalShortcuts"

PORTAL_XML = """
<node>
  <interface name="org.freedesktop.portal.GlobalShortcuts">
    <method name="CreateSession">
      <arg type="a{sv}" name="options" direction="in"/>
      <arg type="o" name="handle" direction="out"/>
    </method>
    <method name="BindShortcuts">
      <arg type="o" name="session_handle" direction="in"/>
      <arg type="a(sa{sv})" name="shortcuts" direction="in"/>
      <arg type="s" name="parent_window" direction="in"/>
      <arg type="a{sv}" name="options" direction="in"/>
      <arg type="o" name="handle" direction="out"/>
    </method>
    <signal name="Activated">
      <arg type="o" name="session_handle"/>
      <arg type="s" name="shortcut_id"/>
      <arg type="t" name="timestamp"/>
      <arg type="a{sv}" name="options"/>
    </signal>
    <property name="version" type="u" access="read"/>
  </interface>
  <interface name="org.freedesktop.host.portal.Registry">
    <method name="Register">
      <arg type="s" name="app_id" direction="in"/>
      <arg type="a{sv}" name="options" direction="in"/>
    </method>
  </interface>
</node>
"""

SESSION_XML = """
<node>
  <interface name="org.freedesktop.portal.Session">
    <method name="Close"/>
  </interface>
</node>
"""

portal_info = Gio.DBusNodeInfo.new_for_xml(PORTAL_XML)
session_info = Gio.DBusNodeInfo.new_for_xml(SESSION_XML).interfaces[0]
conn = Gio.bus_get_sync(Gio.BusType.SESSION, None)
loop = GLib.MainLoop()
state = {"session": None, "client": None, "sessions": {}}


def log(**event):
    sys.stdout.write(json.dumps(event) + "\n")
    sys.stdout.flush()


def sender_path(sender):
    return sender.lstrip(":").replace(".", "_")


def respond(sender, request_path, code, results):
    # The real portal answers after the method returns; so do we.
    def emit():
        conn.emit_signal(sender, request_path, "org.freedesktop.portal.Request", "Response",
                         GLib.Variant("(ua{sv})", (code, results)))
        return False
    GLib.timeout_add(20, emit)


def session_call(_conn, sender, path, _iface, method, _params, invocation):
    log(call="Session." + method, session=path)
    reg = state["sessions"].pop(path, None)
    if reg is not None:
        conn.unregister_object(reg)
    if state["session"] == path:
        state["session"] = None
    invocation.return_value(None)


def portal_call(_conn, sender, _path, iface, method, params, invocation):
    args = params.unpack()
    if iface == "org.freedesktop.host.portal.Registry":
        log(call="Register", app_id=args[0])
        invocation.return_value(None)
        return
    if method == "CreateSession":
        opts = args[0]
        request = "%s/request/%s/%s" % (PATH, sender_path(sender), opts["handle_token"])
        session = "%s/session/%s/%s" % (PATH, sender_path(sender), opts["session_handle_token"])
        log(call="CreateSession", request=request, session=session)
        invocation.return_value(GLib.Variant("(o)", (request,)))
        if MODE == "cancel-session":
            respond(sender, request, 1, {})
            return
        state["sessions"][session] = conn.register_object(session, session_info, session_call, None, None)
        respond(sender, request, 0, {"session_handle": GLib.Variant("s", session)})
        return
    if method == "BindShortcuts":
        session, shortcuts, _parent, opts = args
        request = "%s/request/%s/%s" % (PATH, sender_path(sender), opts["handle_token"])
        log(call="BindShortcuts", session=session,
            shortcuts=[[sid, props.get("preferred_trigger", ""), props.get("description", "")]
                       for sid, props in shortcuts])
        invocation.return_value(GLib.Variant("(o)", (request,)))
        if MODE == "refuse" or session not in state["sessions"]:
            respond(sender, request, 1 if MODE == "refuse" else 2, {})
            return
        state["session"] = session
        state["client"] = sender
        bound = [(sid, {"description": GLib.Variant("s", props.get("description", "")),
                        "trigger_description": GLib.Variant("s", props.get("preferred_trigger", ""))})
                 for sid, props in shortcuts]
        respond(sender, request, 0, {"shortcuts": GLib.Variant("a(sa{sv})", bound)})
        return
    invocation.return_dbus_error("org.freedesktop.DBus.Error.UnknownMethod", method)


def portal_property(_conn, _sender, _path, _iface, name):
    if name == "version":
        return GLib.Variant("u", 1)
    return None


def activate(shortcut_id, session):
    if session is None or state["client"] is None:
        log(error="activate without a bound session")
        return
    conn.emit_signal(None, PATH, GS, "Activated",
                     GLib.Variant("(osta{sv})", (session, shortcut_id, 0, {})))
    log(emitted="Activated", id=shortcut_id, session=session)


def on_stdin(fd, _cond):
    data = os.read(fd, 4096)
    if not data:
        loop.quit()
        return False
    for line in data.decode().splitlines():
        cmd, _, arg = line.strip().partition(" ")
        if cmd == "activate":
            activate(arg, state["session"])
        elif cmd == "activate-foreign":
            activate(arg, PATH + "/session/someone_else/stolen")
    return True


for iface in portal_info.interfaces:
    conn.register_object(PATH, iface, portal_call, portal_property, None)
reply = conn.call_sync("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
                       "RequestName", GLib.Variant("(su)", (BUS, 4)), None,
                       Gio.DBusCallFlags.NONE, -1, None)
if reply.unpack()[0] != 1:
    sys.exit("could not own " + BUS)
GLib.unix_fd_add_full(GLib.PRIORITY_DEFAULT, 0, GLib.IOCondition.IN | GLib.IOCondition.HUP, on_stdin)
log(ready=True)
loop.run()
