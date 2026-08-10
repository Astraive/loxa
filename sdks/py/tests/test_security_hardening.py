import pytest

from loza.core.http_client import CollectorClient
from loza.cortex.client import CortexClient
from loza.core.pipeline import DiskOfflineBuffer
from loza.sinks.file import FileSink


def test_collector_client_rejects_metadata_endpoint():
    with pytest.raises(ValueError, match="non-public"):
        CollectorClient("http://169.254.169.254/latest/meta-data")


def test_file_sink_rejects_parent_traversal():
    with pytest.raises(ValueError, match="traversal"):
        FileSink("../loza.ndjson")


def test_disk_offline_buffer_rejects_parent_traversal():
    with pytest.raises(ValueError, match="traversal"):
        DiskOfflineBuffer("../loza-spool.ndjson")


def test_cortex_client_rejects_metadata_endpoint():
    with pytest.raises(ValueError, match="non-public"):
        CortexClient("http://169.254.169.254/latest/meta-data")
