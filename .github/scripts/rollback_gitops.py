#!/usr/bin/env python3
"""Restore a service manifest to its previous distinct image tag."""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path

SERVICES = {
    "auth-service",
    "flag-service",
    "evaluation-service",
    "analytics-service",
    "targeting-service",
}
IMAGE_LINE = re.compile(r"^(\s*image:\s*)(\S+)(\s*(?:#.*)?)$", re.MULTILINE)


def run_git(root: Path, *args: str) -> str:
    result = subprocess.run(
        ["git", *args], cwd=root, check=True, text=True, capture_output=True
    )
    return result.stdout


def image_in(text: str) -> str:
    matches = IMAGE_LINE.findall(text)
    if len(matches) != 1:
        raise ValueError(f"esperava uma imagem no manifesto; encontrei {len(matches)}")
    return matches[0][1]


def set_output(name: str, value: str) -> None:
    output_path = os.environ.get("GITHUB_OUTPUT")
    if output_path:
        with open(output_path, "a", encoding="utf-8") as output:
            output.write(f"{name}={value}\n")


def rollback(root: Path, service: str, expected_image: str) -> bool:
    if service not in SERVICES:
        raise ValueError("microsserviço não permitido")
    if not expected_image:
        raise ValueError("imagem esperada ausente no evento")

    relative_manifest = Path(".github") / "GitOps" / service / "deployment.yaml"
    manifest = root / relative_manifest
    current_text = manifest.read_text(encoding="utf-8")
    current_image = image_in(current_text)
    if current_image != expected_image:
        print(
            f"Alerta obsoleto para {service}: GitOps usa {current_image}, "
            f"mas o alerta observou {expected_image}; rollback ignorado."
        )
        set_output("changed", "false")
        return False

    commits = run_git(
        root, "log", "--format=%H", "--", relative_manifest.as_posix()
    ).splitlines()
    previous_image = ""
    for commit in commits[1:]:
        prior_text = run_git(root, "show", f"{commit}:{relative_manifest.as_posix()}")
        candidate = image_in(prior_text)
        if candidate != current_image:
            previous_image = candidate
            break

    if not previous_image:
        raise ValueError(f"não encontrei uma imagem anterior distinta para {service}")

    updated_text, replacements = IMAGE_LINE.subn(
        lambda match: f"{match.group(1)}{previous_image}{match.group(3)}",
        current_text,
        count=1,
    )
    if replacements != 1:
        raise ValueError("não foi possível atualizar a imagem no manifesto")

    manifest.write_text(updated_text, encoding="utf-8", newline="")
    print(f"Rollback GitOps preparado: {service} {current_image} -> {previous_image}")
    set_output("changed", "true")
    set_output("manifest", relative_manifest.as_posix())
    set_output("previous_image", previous_image)
    set_output("current_image", current_image)
    return True


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--service", required=True)
    parser.add_argument("--expected-image", required=True)
    parser.add_argument("--repo-root", type=Path, default=Path.cwd())
    args = parser.parse_args()

    try:
        changed = rollback(args.repo_root.resolve(), args.service, args.expected_image)
    except (OSError, subprocess.CalledProcessError, ValueError) as error:
        print(f"Erro no rollback GitOps: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
