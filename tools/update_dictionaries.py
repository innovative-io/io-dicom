#!/usr/bin/env python3
import re
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DICOM_DICT_URL = "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/_dicom_dict.py"
UID_DICT_URL = "https://raw.githubusercontent.com/pydicom/pydicom/main/src/pydicom/_uid_dict.py"


def fetch(url: str) -> str:
    with urllib.request.urlopen(url) as resp:
        return resp.read().decode("utf-8")


def parse_dicom_dictionary(src: str):
    ns = {}
    exec(src, ns)
    return ns["DicomDictionary"]


def parse_uid_dictionary(src: str):
    ns = {}
    exec(src, ns)
    return ns["UID_dictionary"]


def sanitize_identifier(name: str) -> str:
    ident = re.sub(r"[^0-9A-Za-z]+", "", name)
    if not ident:
        ident = "Unknown"
    if ident[0].isdigit():
        ident = f"Tag{ident}"
    return ident


def write_tags_file(path: Path, dictionary: dict) -> None:
    lines = ["package tags", ""]

    used_names = set()
    entries = sorted(
        dictionary.items(),
        key=lambda kv: (((int(kv[0]) >> 16) & 0xFFFF), (int(kv[0]) & 0xFFFF)),
    )
    generated_names = []

    for tag_value, value in entries:
        group = (int(tag_value) >> 16) & 0xFFFF
        element = int(tag_value) & 0xFFFF
        vr, vm, name, _retired, keyword = value
        ident_base = keyword if keyword else sanitize_identifier(name)
        ident = sanitize_identifier(ident_base)
        if ident in used_names:
            ident = f"{ident}{group:04X}{element:04X}"
        used_names.add(ident)
        generated_names.append(ident)

        lines.extend(
            [
                f"// {ident} - ({group:04X},{element:04X}) {name}",
                f"var {ident} = &Tag{{",
                f"\tGroup:\t\t0x{group:04X},",
                f"\tElement:\t0x{element:04X},",
                f"\tVR:\t\t\"{vr}\",",
                f"\tVM:\t\t\"{vm}\",",
                f"\tName:\t\t\"{ident}\",",
                f"\tDescription:\t\"{name}\",",
                "}",
                "",
            ]
        )

    lines.append("var tags = []*Tag{")
    lines.extend([f"\t{name}," for name in generated_names])
    lines.extend(["}", ""])

    path.write_text("\n".join(lines), encoding="utf-8")


def uid_sort_key(uid: str):
    out = []
    for part in uid.split("."):
        try:
            out.append(int(part))
        except ValueError:
            out.append(part)
    return out


def write_transfer_syntax_file(path: Path, dictionary: dict) -> None:
    lines = ["package transfersyntax", ""]

    used_names = set()
    entries = []
    for uid, value in dictionary.items():
        name, uid_type, _info, _retired, keyword = value
        if uid_type != "Transfer Syntax":
            continue

        ident_base = keyword if keyword else sanitize_identifier(name)
        ident = sanitize_identifier(ident_base)
        if ident in used_names:
            ident = f"{ident}{sanitize_identifier(uid)}"
        used_names.add(ident)
        entries.append((uid, name, ident))

    entries.sort(key=lambda item: uid_sort_key(item[0]))
    generated_names = []

    for uid, name, ident in entries:
        generated_names.append(ident)
        lines.extend(
            [
                f"// {ident} - ({uid}) {name}",
                f"var {ident} = &TransferSyntax{{",
                f"\tUID:\t\t\"{uid}\",",
                f"\tName:\t\t\"{ident}\",",
                f"\tDescription:\t\"{name}\",",
                "\tType:\t\t\"Transfer Syntax\",",
                "}",
                "",
            ]
        )

    lines.append("var transferSyntaxes = []*TransferSyntax{")
    lines.extend([f"\t{name}," for name in generated_names])
    lines.extend(["}", ""])

    path.write_text("\n".join(lines), encoding="utf-8")


def main() -> None:
    dicom_dictionary = parse_dicom_dictionary(fetch(DICOM_DICT_URL))
    uid_dictionary = parse_uid_dictionary(fetch(UID_DICT_URL))

    write_tags_file(ROOT / "dictionary/tags/dicom_tags.go", dicom_dictionary)
    write_transfer_syntax_file(ROOT / "dictionary/transfersyntax/transfer_syntaxes.go", uid_dictionary)


if __name__ == "__main__":
    main()
