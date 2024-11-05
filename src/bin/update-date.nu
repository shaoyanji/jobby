#!/usr/bin/env nu
sops set src/letter.enc.yaml '["date"]' (date now | format date '"%d.%m.%Y"')
