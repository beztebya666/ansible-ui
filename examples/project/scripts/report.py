#!/usr/bin/env python3
"""Python "app" demo for ansible-ui — prints host facts in the live terminal.

Run extra-vars are exported as environment variables, so `greeting` becomes
the GREETING env var below.
"""
import os
import platform
import sys

CYAN = "\033[36m"
BOLD = "\033[1m"
GREEN = "\033[32m"
RESET = "\033[0m"


def main() -> int:
    print(f"{BOLD}{CYAN}=== ansible-ui · python app demo ==={RESET}")
    print(f"python   : {sys.version.split()[0]}")
    print(f"platform : {platform.platform()}")
    print(f"cwd      : {os.getcwd()}")
    print(f"greeting : {os.environ.get('GREETING', '<none>')}")
    for i in range(1, 4):
        print(f"{GREEN}▸ step {i} ok{RESET}")
    print(f"{BOLD}done.{RESET}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
