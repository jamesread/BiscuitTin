#!/usr/bin/env python3
"""Writes the biscuittin -script steps for screenshots.ini to stdout.

Each section with title= becomes its presses followed by a shot of
<name>.png in this directory. Run via `make shoot`.
"""
import configparser
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
INI = HERE / "screenshots.ini"


def main():
    cp = configparser.ConfigParser(interpolation=None)
    cp.read(INI)
    out = ["# Generated from screenshots.ini by generate-script.py."]
    out.append("clock %s" % cp.defaults()["clock"].strip())
    last_input = None
    for section in cp.sections():
        if not cp.get(section, "title", fallback="").strip():
            continue
        name = cp.get(section, "name", fallback=section).strip() or section
        inp = cp.get(section, "input").strip()
        if inp != last_input:
            out.append("input %s" % inp)
            last_input = inp
        out.append("# [%s]" % section)
        for button in cp.get(section, "steps").replace(",", " ").split():
            out.append("press %s" % button)
        out.append("shot %s" % (HERE / ("%s.png" % name)))
    sys.stdout.write("\n".join(out) + "\n")


if __name__ == "__main__":
    main()
