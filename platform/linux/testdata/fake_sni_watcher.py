#!/usr/bin/env python3
"""Fake org.kde.StatusNotifierWatcher (with a host), for tests.

Owns org.kde.StatusNotifierWatcher on the session bus, then prints READY.
It plays the panel too: when an item registers, it reads the item's
properties like a StatusNotifierHost would. Events are printed to stdout as
JSON lines. Commands are read from stdin, one per line:

  layout          call GetLayout(0, -1, []) on the item's menu
  click LABEL     send dbusmenu Event(id, "clicked") for the item labelled LABEL
  clickid ID      send Event(ID, "clicked") for a raw id
  activate        call Activate(0, 0) on the item (left click)
  activateat X Y  call Activate(X, Y) on the item
  props           re-read the item's properties

Exits when stdin closes.
"""
import json
import os
import sys

from gi.repository import Gio, GLib

BUS = "org.kde.StatusNotifierWatcher"
PATH = "/StatusNotifierWatcher"
ITEM_PATH = "/StatusNotifierItem"
ITEM_IFACE = "org.kde.StatusNotifierItem"
MENU_IFACE = "com.canonical.dbusmenu"

XML = """
<node>
  <interface name="org.kde.StatusNotifierWatcher">
    <method name="RegisterStatusNotifierItem">
      <arg type="s" name="service" direction="in"/>
    </method>
    <method name="RegisterStatusNotifierHost">
      <arg type="s" name="service" direction="in"/>
    </method>
    <property name="RegisteredStatusNotifierItems" type="as" access="read"/>
    <property name="IsStatusNotifierHostRegistered" type="b" access="read"/>
    <property name="ProtocolVersion" type="i" access="read"/>
    <signal name="StatusNotifierItemRegistered"><arg type="s"/></signal>
    <signal name="StatusNotifierItemUnregistered"><arg type="s"/></signal>
    <signal name="StatusNotifierHostRegistered"/>
  </interface>
</node>
"""

conn = Gio.bus_get_sync(Gio.BusType.SESSION, None)
loop = GLib.MainLoop()
state = {"item": None, "menu": None, "layout": {}}


def log(**event):
    sys.stdout.write(json.dumps(event) + "\n")
    sys.stdout.flush()


def call(dest, path, iface, method, args, reply):
    return conn.call_sync(dest, path, iface, method, args,
                          GLib.VariantType.new(reply) if reply else None,
                          Gio.DBusCallFlags.NONE, 5000, None)


def read_props():
    item = state["item"]
    props = call(item, ITEM_PATH, "org.freedesktop.DBus.Properties", "GetAll",
                 GLib.Variant("(s)", (ITEM_IFACE,)), "(a{sv})").unpack()[0]
    state["menu"] = props.get("Menu")
    tooltip = props.get("ToolTip")
    pixmaps = props.get("IconPixmap", [])
    log(props={
        "XAyatanaLabel": props.get("XAyatanaLabel"),
        "IconPixmapSize": [pixmaps[0][0], pixmaps[0][1]] if pixmaps else None,
        "IconPixmapData": list(pixmaps[0][2]) if pixmaps else None,
        "Id": props.get("Id"), "Title": props.get("Title"), "Status": props.get("Status"),
        "Category": props.get("Category"), "IconName": props.get("IconName"),
        "IconPixmap": len(props.get("IconPixmap", [])), "Menu": props.get("Menu"),
        "ItemIsMenu": props.get("ItemIsMenu"),
        "ToolTip": tooltip[2] if tooltip else None,
    })


def node_json(node):
    nid, props, children = node
    return {"id": nid, "label": props.get("label"), "enabled": props.get("enabled"),
            "type": props.get("type"), "toggle-type": props.get("toggle-type"),
            "toggle-state": props.get("toggle-state"),
            "visible": props.get("visible"), "children-display": props.get("children-display"),
            "children": [node_json(c) for c in children]}


def get_layout():
    rev, root = call(state["item"], state["menu"], MENU_IFACE, "GetLayout",
                     GLib.Variant("(iias)", (0, -1, [])), "(u(ia{sv}av))").unpack()
    tree = node_json(root)
    state["layout"] = {c["label"]: c["id"] for c in tree["children"]}
    log(layout=tree, revision=rev)


def menu_event(item_id):
    try:
        call(state["item"], state["menu"], MENU_IFACE, "Event",
             GLib.Variant("(isvu)", (item_id, "clicked", GLib.Variant("i", 0), 0)), None)
        log(event="clicked", id=item_id, ok=True)
    except GLib.Error as e:
        log(event="clicked", id=item_id, ok=False, error=e.message)


def on_call(_conn, sender, _path, _iface, method, params, invocation):
    if method == "RegisterStatusNotifierItem":
        service = params.unpack()[0]
        # Items register either a bus name or an object path on the sender.
        state["item"] = sender if service.startswith("/") else service
        log(call="RegisterStatusNotifierItem", service=service, sender=sender)
        invocation.return_value(None)
        conn.emit_signal(None, PATH, BUS, "StatusNotifierItemRegistered",
                         GLib.Variant("(s)", (service,)))

        def host_reads():
            read_props()
            return False
        GLib.idle_add(host_reads)
        return
    invocation.return_value(None)


def on_property(_conn, _sender, _path, _iface, name):
    if name == "IsStatusNotifierHostRegistered":
        return GLib.Variant("b", True)
    if name == "ProtocolVersion":
        return GLib.Variant("i", 0)
    if name == "RegisteredStatusNotifierItems":
        return GLib.Variant("as", [state["item"]] if state["item"] else [])
    return None


def on_name_owner_changed(_conn, _sender, _path, _iface, _signal, params):
    name, old, new = params.unpack()
    if state["item"] and name == state["item"] and old and not new:
        log(unregistered=name)
        state["item"] = None


def on_layout_updated(_conn, _sender, _path, _iface, _signal, params):
    rev, parent = params.unpack()
    log(signal="LayoutUpdated", revision=rev, parent=parent)


def on_item_signal(_conn, _sender, _path, _iface, signal, params):
    log(signal=signal, args=list(params.unpack()))


def on_stdin(fd, _cond):
    data = os.read(fd, 4096)
    if not data:
        loop.quit()
        return False
    for line in data.decode().splitlines():
        cmd, _, arg = line.strip().partition(" ")
        if cmd == "layout":
            get_layout()
        elif cmd == "click":
            menu_event(state["layout"][arg])
        elif cmd == "clickid":
            menu_event(int(arg))
        elif cmd == "activateat":
            x, y = (int(v) for v in arg.split())
            call(state["item"], ITEM_PATH, ITEM_IFACE, "Activate", GLib.Variant("(ii)", (x, y)), None)
            log(called="Activate", x=x, y=y)
        elif cmd == "activate":
            call(state["item"], ITEM_PATH, ITEM_IFACE, "Activate", GLib.Variant("(ii)", (0, 0)), None)
            log(called="Activate")
        elif cmd == "props":
            read_props()
    return True


info = Gio.DBusNodeInfo.new_for_xml(XML).interfaces[0]
conn.register_object(PATH, info, on_call, on_property, None)
conn.signal_subscribe("org.freedesktop.DBus", "org.freedesktop.DBus", "NameOwnerChanged",
                      "/org/freedesktop/DBus", None, Gio.DBusSignalFlags.NONE, on_name_owner_changed)
conn.signal_subscribe(None, MENU_IFACE, "LayoutUpdated", None, None,
                      Gio.DBusSignalFlags.NONE, on_layout_updated)
conn.signal_subscribe(None, ITEM_IFACE, None, ITEM_PATH, None,
                      Gio.DBusSignalFlags.NONE, on_item_signal)
reply = conn.call_sync("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
                       "RequestName", GLib.Variant("(su)", (BUS, 4)), None,
                       Gio.DBusCallFlags.NONE, -1, None)
if reply.unpack()[0] != 1:
    sys.exit("could not own " + BUS)
GLib.unix_fd_add_full(GLib.PRIORITY_DEFAULT, 0, GLib.IOCondition.IN | GLib.IOCondition.HUP, on_stdin)
log(ready=True)
loop.run()
