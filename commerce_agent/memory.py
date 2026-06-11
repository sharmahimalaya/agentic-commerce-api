import json
import os
import time
from typing import List, Dict, Any

BASE_DIR = os.path.dirname(__file__)
MEMORY_DIR = os.path.join(BASE_DIR, ".cache")
MEMORY_FILE = os.path.join(MEMORY_DIR, "memory.json")
MAX_ENTRIES = 500


def _ensure_dir() -> None:
    if not os.path.exists(MEMORY_DIR):
        os.makedirs(MEMORY_DIR, exist_ok=True)


def load_memory() -> List[Dict[str, Any]]:
    _ensure_dir()
    if not os.path.exists(MEMORY_FILE):
        return []
    try:
        with open(MEMORY_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return []


def save_memory(entries: List[Dict[str, Any]]) -> None:
    _ensure_dir()
    try:
        with open(MEMORY_FILE, "w", encoding="utf-8") as f:
            json.dump(entries[-MAX_ENTRIES:], f, ensure_ascii=False, indent=2)
    except Exception:
        # best-effort only
        pass


def add(entry: Dict[str, Any]) -> None:
    entries = load_memory()
    entry.setdefault("ts", int(time.time()))
    entries.append(entry)
    save_memory(entries)


def recent(n: int = 5) -> List[str]:
    entries = load_memory()
    out: List[str] = []
    for e in entries[-n:]:
        role = e.get("role") or e.get("r") or "note"
        text = e.get("text") or e.get("t") or str(e)
        out.append(f"[{role}] {text}")
    return out
