"""Certificate state survives container replacement; use a disposable local CA."""
import time
import docker
import pytest
import urllib3
from testcontainers.core.container import DockerContainer
from conftest import ROOT


@pytest.fixture
def certificate_volume(request):
    client = docker.from_env()
    volume = client.volumes.create()
    request.addfinalizer(lambda: volume.remove(force=True))
    return volume


def start_site(request, network, configuration, volume):
    container = DockerContainer("nossa-inflacao-web:verify")
    container.with_network(network).with_exposed_ports(443)
    container.with_env("PUBLIC_DOMAIN", "site.test")
    container.with_volume_mapping(str(configuration), "/etc/caddy/Caddyfile", mode="ro")
    container.with_volume_mapping(volume.name, "/data", mode="rw")
    container.with_tmpfs_mount("/tmp")
    container.with_kwargs(cap_drop=["ALL"], cap_add=["NET_BIND_SERVICE"], read_only=True)
    request.addfinalizer(lambda: stop_site(container))
    container.start()
    return container


def check_https(container):
    connection = urllib3.HTTPSConnectionPool(
        container.get_container_host_ip(), port=int(container.get_exposed_port(443)),
        server_hostname="site.test", cert_reqs="CERT_NONE", assert_hostname=False)
    await_https(connection, container, 30)


def await_https(connection, container, attempts):
    if try_https(connection):
        return
    if attempts == 0:
        raise AssertionError(container.get_logs())
    time.sleep(1)
    await_https(connection, container, attempts - 1)


def try_https(connection):
    try:
        response = connection.request("GET", "/", headers={"Host": "site.test"}, retries=False)
        return response.status == 200
    except (urllib3.exceptions.HTTPError, OSError):
        return False


def test_tls_state_survives_replacement(request, network, core, tmp_path, certificate_volume):
    config = (ROOT / "deployment/web/Caddyfile").read_text().replace("{\n", "{\n    local_certs\n", 1)
    path = tmp_path / "Caddyfile"
    path.write_text(config)
    path.chmod(0o644)
    roots = []
    for attempt in range(2):
        container = start_site(request, network, path, certificate_volume)
        check_https(container)
        wrapped = container.get_wrapped_container()
        result = wrapped.exec_run(["cat", "/data/caddy/pki/authorities/local/root.crt"])
        assert result.exit_code == 0
        roots.append(result.output)
        stop_site(container)
    assert roots[0] == roots[1]


def stop_site(container):
    try:
        container.stop()
    except docker.errors.NotFound:
        pass
