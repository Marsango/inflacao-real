"""Encrypted, explicit recovery; application writers are paused for snapshots."""
import getpass
import shutil
import tarfile
import tempfile
from pathlib import Path
from runtime import CONFIG, command, compose, revision, require_clean_checkout
from lifecycle import database, start_application
from settings import bootstrap

MEMBERS = {"database.dump", "spool.tar.gz", "production.env", "scrapers.toml", "revision"}


def crypt(source, destination, decrypt=False):
    password = getpass.getpass("Backup passphrase: ")
    if not password:
        raise RuntimeError("An encryption passphrase is required")
    if not decrypt and password != getpass.getpass("Confirm passphrase: "):
        raise RuntimeError("Passphrases do not match")
    arguments = ["openssl", "enc", "-aes-256-cbc", "-pbkdf2", "-iter", "600000", "-salt"]
    if decrypt:
        arguments.append("-d")
    command(*arguments, "-in", str(source), "-out", str(destination), "-pass", "stdin",
            input=(password + "\n").encode())


def snapshot(directory):
    compose("stop", "scrapers", "core")
    try:
        dump_data(directory)
    finally:
        start_application()
    for name in ["production.env", "scrapers.toml", "revision"]:
        shutil.copy2(CONFIG / name, directory / name)


def dump_data(directory):
    with (directory / "database.dump").open("wb") as destination:
        database("pg_dump", "-U", "nossa_inflacao", "-d", "nossa_inflacao",
                 "--format=custom", stdout=destination)
    with (directory / "spool.tar.gz").open("wb") as destination:
        compose("run", "--rm", "--no-deps", "-T", "scrapers", "tar", "-czf", "-",
                "-C", "/app/data", ".", stdout=destination)


def backup(destination):
    if destination.exists():
        raise RuntimeError("Backup destination already exists")
    with tempfile.TemporaryDirectory(prefix="nossa-backup-") as temporary:
        directory = Path(temporary)
        snapshot(directory)
        archive = directory / "backup.tar"
        with tarfile.open(archive, "w") as bundle:
            for name in sorted(MEMBERS):
                bundle.add(directory / name, arcname=name)
        encrypt_backup(archive, destination)
    print("Encrypted backup created; copy it off the VPS:", destination)


def encrypt_backup(archive, destination):
    try:
        with destination.open("xb"):
            pass
        crypt(archive, destination)
    except BaseException:
        destination.unlink(missing_ok=True)
        raise


def extract_backup(source, directory):
    archive = directory / "backup.tar"
    crypt(source, archive, decrypt=True)
    with tarfile.open(archive) as bundle:
        members = bundle.getmembers()
        if len(members) != len(MEMBERS) or {item.name for item in members} != MEMBERS:
            raise RuntimeError("Unexpected backup members")
        if not all(item.isfile() for item in members):
            raise RuntimeError("Backup members must be regular files")
        bundle.extractall(directory, filter="data")
    expected = (directory / "revision").read_text().strip()
    if expected != revision():
        raise RuntimeError("Check out backup Git revision before restoring: " + expected)


def require_empty_installation():
    if CONFIG.exists() and any(CONFIG.iterdir()):
        raise RuntimeError("Restore requires an empty configuration directory; do not run up first")
    for name in ["postgres_data", "scraper_data"]:
        result = command("docker", "volume", "ls", "--format", "{{.Name}}",
                         capture_output=True, text=True)
        if "nossa-inflacao_" + name in result.stdout.splitlines():
            raise RuntimeError("Restore refuses existing production volumes")


def restore_data(directory):
    compose("build", "--pull")
    compose("up", "-d", "--wait", "postgres")
    with (directory / "database.dump").open("rb") as source:
        database("pg_restore", "-U", "nossa_inflacao", "-d", "nossa_inflacao",
                 "--exit-on-error", "--single-transaction", stdin=source)
    with (directory / "spool.tar.gz").open("rb") as source:
        compose("run", "--rm", "--no-deps", "-T", "scrapers", "tar", "-xzf", "-",
                "--no-same-owner", "-C", "/app/data", stdin=source)


def restore(source):
    require_clean_checkout()
    bootstrap()
    require_empty_installation()
    with tempfile.TemporaryDirectory(prefix="nossa-restore-") as temporary:
        directory = Path(temporary)
        extract_backup(source, directory)
        CONFIG.mkdir(mode=0o700, parents=True, exist_ok=True)
        for name in ["production.env", "scrapers.toml", "revision"]:
            shutil.copy2(directory / name, CONFIG / name)
        (CONFIG / "scrapers.toml").chmod(0o644)
        restore_data(directory)
        start_application()
