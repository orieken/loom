#!/usr/bin/env python3
"""Generate .roomodes YAML from shared/agents/*.md for Roo Code (Epic 42).

Usage: generate-roomodes.py <shared-agents-dir>
Writes YAML to stdout. Called by scripts/generate-configs.sh.
"""

import os
import re
import sys


READ_TOOLS = {"Read", "Glob", "Grep", "WebFetch", "WebSearch"}
WRITE_TOOLS = {"Write", "Edit", "MultiEdit", "NotebookEdit"}
COMMAND_TOOLS = {"Bash"}
MCP_TOOLS = {"Agent", "Artifact", "TaskCreate", "TaskUpdate", "TaskGet",
             "TaskList", "TaskStop", "TaskOutput", "SendMessage"}


def infer_groups(tools_raw: str) -> list:
    if tools_raw.strip() == "*":
        return ["read", "edit", "command", "mcp", "browser"]
    tools = {t.strip() for t in tools_raw.split(",")}
    groups = []
    if tools & READ_TOOLS:
        groups.append("read")
    if tools & WRITE_TOOLS:
        groups.append("edit")
    if tools & COMMAND_TOOLS:
        groups.append("command")
    if tools & MCP_TOOLS or any(t.startswith("mcp__") for t in tools):
        groups.append("mcp")
    return groups or ["read"]


def parse_agent(path: str) -> dict | None:
    with open(path) as f:
        content = f.read()

    parts = content.split("---\n", 2)
    if len(parts) < 3:
        return None

    fm = parts[1]
    body = parts[2].strip()

    name_m = re.search(r"^name:\s*(.+)$", fm, re.MULTILINE)
    desc_m = re.search(r"^description:\s*(.+)$", fm, re.MULTILINE)
    tools_m = re.search(r"^tools:\s*(.+)$", fm, re.MULTILINE)

    if not name_m:
        return None

    slug = name_m.group(1).strip()
    description = desc_m.group(1).strip() if desc_m else ""
    tools_raw = tools_m.group(1).strip() if tools_m else ""
    groups = infer_groups(tools_raw)

    display_name = " ".join(w.capitalize() for w in slug.replace("-", " ").split())

    return {
        "slug": slug,
        "name": display_name,
        "description": description,
        "roleDefinition": body,
        "whenToUse": description,
        "groups": groups,
    }


def yaml_str(value: str) -> str:
    escaped = value.replace("\\", "\\\\").replace('"', '\\"')
    return f'"{escaped}"'


def emit_modes(agents_dir: str) -> None:
    filenames = sorted(
        f for f in os.listdir(agents_dir)
        if f.endswith(".md") and f != "CHANGELOG.md"
    )

    print("customModes:")
    for filename in filenames:
        agent = parse_agent(os.path.join(agents_dir, filename))
        if not agent:
            continue

        print(f"  - slug: {agent['slug']}")
        print(f"    name: {agent['name']}")
        print(f"    description: {yaml_str(agent['description'])}")
        print(f"    whenToUse: {yaml_str(agent['whenToUse'])}")
        print(f"    roleDefinition: |")
        for line in agent["roleDefinition"].splitlines():
            print(f"      {line}")
        print(f"    groups:")
        for group in agent["groups"]:
            print(f"      - {group}")
        print()


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <shared-agents-dir>", file=sys.stderr)
        sys.exit(1)

    emit_modes(sys.argv[1])
