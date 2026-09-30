# Credentials

This file sits in `.private/`. The grant denies that folder, so the page cannot read or write it, even though it is inside the vault.

Deny rules win over allow rules, and ignore letter case, so `.PRIVATE` is refused too. 
