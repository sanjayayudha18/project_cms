"""SHA-256 checksum helper (task 5.1)."""
from __future__ import annotations

import hashlib
from pathlib import Path

_CHUNK_SIZE = 1024 * 1024  # 1MB


def compute_checksum(path: Path) -> str:
    """SHA-256 checksum of a file's contents, read in 1MB chunks."""
    digest = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(_CHUNK_SIZE), b""):
            digest.update(chunk)
    return digest.hexdigest()


def demo() -> None:
    import tempfile

    with tempfile.NamedTemporaryFile(delete=False) as f:
        f.write(b"hello world")
        path = Path(f.name)
    try:
        checksum = compute_checksum(path)
        assert checksum == hashlib.sha256(b"hello world").hexdigest()
        assert len(checksum) == 64
    finally:
        path.unlink()
    print("checksum.py demo OK")


if __name__ == "__main__":
    demo()
