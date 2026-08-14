"""Tests for Symbol class."""

import pytest
from unittest.mock import Mock
from pysysml.symbol import Symbol
from pysysml.proto import sysml_pb2


class TestSymbolBasicProperties:
    """Test basic Symbol properties: id, name, kind."""

    def test_symbol_properties(self):
        """Test that Symbol exposes basic properties from SymbolInfo."""
        # Create mock client
        mock_client = Mock()
        
        # Create SymbolInfo protobuf with metadata
        info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            metadata={"source": "test.sysml", "line": "5"}
        )
        
        # Create Symbol
        sym = Symbol(info, mock_client, "model_abc123")
        
        # Verify properties
        assert sym.id == "Vehicles::Vehicle"
        assert sym.name == "Vehicle"
        assert sym.kind == "partDef"
        assert sym.metadata == {"source": "test.sysml", "line": "5"}
        assert isinstance(sym.metadata, dict)
    
    def test_symbol_without_client(self):
        """Test that Symbol works without client (for Model root symbol)."""
        # Create SymbolInfo protobuf
        info = sysml_pb2.SymbolInfo(
            id="Model",
            name="Model",
            kind="package"
        )
        
        # Create Symbol without client
        sym = Symbol(info, None, "model_abc123")
        
        # Verify properties still work
        assert sym.id == "Model"
        assert sym.name == "Model"
        assert sym.kind == "package"
    
    def test_symbol_str(self):
        """Test __str__ returns 'name (kind)' format."""
        info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef"
        )
        sym = Symbol(info, None, "model_abc123")
        
        # Verify __str__ format
        assert str(sym) == "Vehicle (partDef)"


class TestSymbolChildren:
    """Test Symbol.children() method."""

    def test_symbol_children_lazy_loading(self):
        """Test children() returns Symbol objects for child_ids."""
        # Create mock client
        mock_client = Mock()
        
        # Mock get_symbol to return child SymbolInfo objects
        child1_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::mass",
            name="mass",
            kind="attributeUsage"
        )
        child2_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::speed",
            name="speed",
            kind="attributeUsage"
        )
        mock_client.get_symbol.side_effect = [child1_info, child2_info]
        
        # Create parent SymbolInfo with child_ids
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=["Vehicles::Vehicle::mass", "Vehicles::Vehicle::speed"]
        )
        
        # Create parent Symbol
        parent = Symbol(parent_info, mock_client, "model_abc123")
        
        # Get children
        children = parent.children()
        
        # Verify
        assert len(children) == 2
        assert children[0].id == "Vehicles::Vehicle::mass"
        assert children[0].name == "mass"
        assert children[1].id == "Vehicles::Vehicle::speed"
        assert children[1].name == "speed"
        
        # Verify RPC calls with model_hash
        assert mock_client.get_symbol.call_count == 2
        mock_client.get_symbol.assert_any_call("model_abc123", "Vehicles::Vehicle::mass")
        mock_client.get_symbol.assert_any_call("model_abc123", "Vehicles::Vehicle::speed")
    
    def test_symbol_children_caching(self):
        """Test children() caches results and doesn't hit client twice."""
        # Create mock client
        mock_client = Mock()
        
        # Mock get_symbol to return child
        child_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::mass",
            name="mass",
            kind="attributeUsage"
        )
        mock_client.get_symbol.return_value = child_info
        
        # Create parent SymbolInfo with one child
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=["Vehicles::Vehicle::mass"]
        )
        
        # Create parent Symbol
        parent = Symbol(parent_info, mock_client, "model_abc123")
        
        # First call - should fetch from client
        children1 = parent.children()
        assert len(children1) == 1
        assert mock_client.get_symbol.call_count == 1
        
        # Second call - should return cached, no additional RPC
        children2 = parent.children()
        assert len(children2) == 1
        assert mock_client.get_symbol.call_count == 1  # Still 1, not 2
        assert children1 is children2  # Same list object
    
    def test_symbol_children_empty(self):
        """Test children() with empty child_ids returns empty list."""
        # Create mock client
        mock_client = Mock()
        
        # Create SymbolInfo with no child_ids
        info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=[]
        )
        
        # Create Symbol
        sym = Symbol(info, mock_client, "model_abc123")
        
        # Get children - should return empty list
        children = sym.children()
        
        assert children == []
        assert mock_client.get_symbol.call_count == 0  # No RPC calls
    
    def test_children_without_client(self):
        """Test children() returns empty list when no client."""
        # Create SymbolInfo with child_ids but no client
        info = sysml_pb2.SymbolInfo(
            id="Model",
            name="Model",
            kind="package",
            child_ids=["Model::Vehicles", "Model::Parts"]
        )
        
        # Create Symbol without client
        sym = Symbol(info, None, "model_abc123")
        
        # Get children - should return empty list
        children = sym.children()
        
        assert children == []
    
    def test_children_skips_none_results(self):
        """Test children() skips None when get_symbol fails."""
        # Create mock client that returns None for second child
        mock_client = Mock()
        
        child1_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::mass",
            name="mass",
            kind="attributeUsage"
        )
        # Second call returns None (symbol not found or RPC error)
        mock_client.get_symbol.side_effect = [child1_info, None]
        
        # Create parent SymbolInfo
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=["Vehicles::Vehicle::mass", "Vehicles::Vehicle::invalid"]
        )
        
        # Create parent Symbol
        parent = Symbol(parent_info, mock_client, "model_abc123")
        
        # Get children - should skip None
        children = parent.children()
        
        # Verify only valid child returned
        assert len(children) == 1
        assert children[0].id == "Vehicles::Vehicle::mass"


class TestSymbolAttributes:
    """Test Symbol.attributes(), parts() and get_attr() against real service kinds."""

    def _vehicle(self, mock_client):
        """Build a Vehicle symbol whose children use the service's kind strings."""
        children = [
            sysml_pb2.SymbolInfo(id="Vehicles::Vehicle::mass", name="mass", kind="attributeUsage"),
            sysml_pb2.SymbolInfo(id="Vehicles::Vehicle::engine", name="engine", kind="partUsage"),
            sysml_pb2.SymbolInfo(id="Vehicles::Vehicle::Wheel", name="Wheel", kind="partDef"),
            sysml_pb2.SymbolInfo(id="Vehicles::Vehicle::Color", name="Color", kind="attributeDef"),
            sysml_pb2.SymbolInfo(id="Vehicles::Vehicle::cargo", name="cargo", kind="itemUsage"),
        ]
        mock_client.get_symbol.side_effect = children
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="partDef",
            child_ids=[child.id for child in children],
        )
        return Symbol(parent_info, mock_client, "model_abc123")

    def test_attributes_matches_service_kinds(self):
        """attributes() selects attributeUsage/attributeDef children."""
        parent = self._vehicle(Mock())

        attributes = parent.attributes()

        assert [attr.name for attr in attributes] == ["mass", "Color"]

    def test_parts_matches_service_kinds(self):
        """parts() selects partUsage/partDef children."""
        parent = self._vehicle(Mock())

        parts = parent.parts()

        assert [part.name for part in parts] == ["engine", "Wheel"]

    def test_get_attr_finds_attribute(self):
        """get_attr() returns the named attribute, None otherwise."""
        parent = self._vehicle(Mock())

        assert parent.get_attr("mass").kind == "attributeUsage"
        assert parent.get_attr("engine") is None
        assert parent.get_attr("missing") is None

    def test_kind_matching_is_case_insensitive(self):
        """Kinds are matched case-insensitively, so PascalCase producers work."""
        mock_client = Mock()
        mock_client.get_symbol.side_effect = [
            sysml_pb2.SymbolInfo(id="P::mass", name="mass", kind="AttributeUsage"),
            sysml_pb2.SymbolInfo(id="P::engine", name="engine", kind="PartUsage"),
        ]
        parent = Symbol(
            sysml_pb2.SymbolInfo(id="P", name="P", kind="PartDef", child_ids=["P::mass", "P::engine"]),
            mock_client,
            "model_abc123",
        )

        assert [attr.name for attr in parent.attributes()] == ["mass"]
        assert [part.name for part in parent.parts()] == ["engine"]
