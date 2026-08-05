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
            kind="PartDef",
            metadata={"source": "test.sysml", "line": "5"}
        )
        
        # Create Symbol
        sym = Symbol(info, mock_client, "model_abc123")
        
        # Verify properties
        assert sym.id == "Vehicles::Vehicle"
        assert sym.name == "Vehicle"
        assert sym.kind == "PartDef"
        assert sym.metadata == {"source": "test.sysml", "line": "5"}
        assert isinstance(sym.metadata, dict)
    
    def test_symbol_without_client(self):
        """Test that Symbol works without client (for Model root symbol)."""
        # Create SymbolInfo protobuf
        info = sysml_pb2.SymbolInfo(
            id="Model",
            name="Model",
            kind="Package"
        )
        
        # Create Symbol without client
        sym = Symbol(info, None, "model_abc123")
        
        # Verify properties still work
        assert sym.id == "Model"
        assert sym.name == "Model"
        assert sym.kind == "Package"
    
    def test_symbol_str(self):
        """Test __str__ returns 'name (kind)' format."""
        info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="PartDef"
        )
        sym = Symbol(info, None, "model_abc123")
        
        # Verify __str__ format
        assert str(sym) == "Vehicle (PartDef)"


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
            kind="AttributeUsage"
        )
        child2_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::speed",
            name="speed",
            kind="AttributeUsage"
        )
        mock_client.get_symbol.side_effect = [child1_info, child2_info]
        
        # Create parent SymbolInfo with child_ids
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="PartDef",
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
            kind="AttributeUsage"
        )
        mock_client.get_symbol.return_value = child_info
        
        # Create parent SymbolInfo with one child
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="PartDef",
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
            kind="PartDef",
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
            kind="Package",
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
            kind="AttributeUsage"
        )
        # Second call returns None (symbol not found or RPC error)
        mock_client.get_symbol.side_effect = [child1_info, None]
        
        # Create parent SymbolInfo
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="PartDef",
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
    """Test Symbol.attributes() method."""

    def test_symbol_attributes_filtering(self):
        """Test attributes() filters children with 'Attribute' in kind."""
        # Create mock client
        mock_client = Mock()
        
        # Mock get_symbol to return mixed children
        attr1_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::mass",
            name="mass",
            kind="AttributeUsage"
        )
        part_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::engine",
            name="engine",
            kind="PartUsage"
        )
        attr2_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::length",
            name="length",
            kind="PartAttributeUsage"  # Contains "Attribute"
        )
        attr3_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle::year",
            name="year",
            kind="AttributeDef"  # Contains "Attribute"
        )
        
        mock_client.get_symbol.side_effect = [attr1_info, part_info, attr2_info, attr3_info]
        
        # Create parent SymbolInfo
        parent_info = sysml_pb2.SymbolInfo(
            id="Vehicles::Vehicle",
            name="Vehicle",
            kind="PartDef",
            child_ids=[
                "Vehicles::Vehicle::mass",
                "Vehicles::Vehicle::engine",
                "Vehicles::Vehicle::length",
                "Vehicles::Vehicle::year"
            ]
        )
        
        # Create parent Symbol
        parent = Symbol(parent_info, mock_client, "model_abc123")
        
        # Get attributes - should filter by "Attribute" in kind
        attributes = parent.attributes()
        
        # Verify only symbols with "Attribute" in kind
        assert len(attributes) == 3
        assert attributes[0].id == "Vehicles::Vehicle::mass"
        assert attributes[0].kind == "AttributeUsage"
        assert attributes[1].id == "Vehicles::Vehicle::length"
        assert attributes[1].kind == "PartAttributeUsage"
        assert attributes[2].id == "Vehicles::Vehicle::year"
        assert attributes[2].kind == "AttributeDef"
