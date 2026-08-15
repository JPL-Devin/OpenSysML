"""Tests for reading a host:port address written as the host.

``connect("localhost:50123")`` names an address, and used to build the target
``localhost:50123:50051`` and report the service unreachable — which sends a
reader to the service rather than to the call.
"""

import pytest
from unittest.mock import Mock, patch

import pysysml
from pysysml.connection import DEFAULT_PORT, Connection, split_target


class TestSplitTarget:
    def test_a_host_alone_keeps_the_port_given(self):
        assert split_target("localhost") == ("localhost", DEFAULT_PORT)
        assert split_target("example.internal", 50123) == (
            "example.internal", 50123
        )

    def test_an_address_names_its_own_port(self):
        assert split_target("localhost:50123") == ("localhost", 50123)

    def test_an_address_agreeing_with_the_port_given_is_accepted(self):
        assert split_target("localhost:50123", 50123) == ("localhost", 50123)

    def test_an_address_disagreeing_with_the_port_given_is_rejected(self):
        with pytest.raises(ValueError) as exc_info:
            split_target("localhost:50123", 50999)
        assert "different ports" in str(exc_info.value)

    def test_an_unreadable_port_names_the_mistake(self):
        with pytest.raises(ValueError) as exc_info:
            split_target("localhost:grpc")
        assert "localhost:grpc" in str(exc_info.value)
        assert "separately" in str(exc_info.value)

    def test_a_bare_ipv6_address_is_a_host(self):
        # Its colons are the address's own, so none of them names a port.
        assert split_target("::1") == ("::1", DEFAULT_PORT)
        assert split_target("fe80::1", 50123) == ("fe80::1", 50123)

    def test_a_bracketed_ipv6_address_names_its_port(self):
        assert split_target("[::1]:50123") == ("[::1]", 50123)
        assert split_target("[::1]") == ("[::1]", DEFAULT_PORT)


class TestConnectionTarget:
    def _connection(self, *args, **kwargs):
        stub = Mock()
        with patch('grpc.insecure_channel'):
            with patch(
                'pysysml.proto.sysml_pb2_grpc.SysMLServiceStub',
                return_value=stub,
            ):
                return Connection(*args, auto_start=False, **kwargs)

    def test_an_address_connects_to_the_port_it_names(self):
        conn = self._connection("localhost:50123")
        assert (conn.host, conn.port) == ("localhost", 50123)
        assert conn._address == "localhost:50123"

    def test_connect_reads_an_address_too(self):
        with patch('grpc.insecure_channel'):
            with patch('pysysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
                conn = pysysml.connect("localhost:50123", auto_start=False)
        assert (conn.host, conn.port) == ("localhost", 50123)

    def test_a_disagreeing_port_is_rejected_before_any_call(self):
        with pytest.raises(ValueError):
            self._connection("localhost:50123", 50999)


class TestModuleHelpersReadAnAddress:
    """The module-level helpers taking host/port read an address the same way."""

    def setup_method(self):
        pysysml._default_connection = None
        pysysml._default_connection_params = None

    def teardown_method(self):
        pysysml._default_connection = None
        pysysml._default_connection_params = None

    def test_helpers_reject_a_disagreeing_port(self):
        for call in (
            lambda: pysysml.load("demo.sysml", "localhost:50123", 50999),
            lambda: pysysml.eval("1+1", model_hash="h", host="localhost:50123",
                                 port=50999),
            lambda: pysysml.instantiate("Demo::v", model_hash="h",
                                        host="localhost:50123", port=50999),
            lambda: pysysml.convert("sysml", model_hash="h",
                                    host="localhost:50123", port=50999),
        ):
            with pytest.raises(ValueError):
                call()

    def test_an_address_reaches_the_port_it_names(self):
        with patch('pysysml.Connection') as connection:
            pysysml.load("demo.sysml", "localhost:50123")
        assert connection.call_args[0] == ("localhost", 50123)
