import pytest

from loxa.core.http_client import CollectorClient
from loxa.cortex.client import CortexClient
from loxa.core.pipeline import DiskOfflineBuffer
from loxa.sinks.file import FileSink


def test_collector_client_rejects_metadata_endpoint():
    with pytest.raises(ValueError, match="non-public"):
        CollectorClient("http://169.254.169.254/latest/meta-data")


def test_file_sink_rejects_parent_traversal():
    with pytest.raises(ValueError, match="traversal"):
        FileSink("../loxa.ndjson")


def test_disk_offline_buffer_rejects_parent_traversal():
    with pytest.raises(ValueError, match="traversal"):
        DiskOfflineBuffer("../loxa-spool.ndjson")


def test_cortex_client_rejects_metadata_endpoint():
    with pytest.raises(ValueError, match="non-public"):
        CortexClient("http://169.254.169.254/latest/meta-data")
