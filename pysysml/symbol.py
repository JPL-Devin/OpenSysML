"""Symbol class for navigating the semantic model."""

from typing import List, Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from pysysml.proto import sysml_pb2


class Symbol:
    """Represents a symbol in the SysML model (definition or usage).
    
    Wraps a SymbolInfo protobuf message and provides lazy navigation
    through the symbol tree via an optional RPC client.
    
    Attributes:
        id: Unique symbol identifier (fully-qualified name)
        name: Simple name of the symbol
        kind: SysML element kind (e.g., "PartDef", "AttributeUsage")
    """
    
    def __init__(self, pb_symbol: "sysml_pb2.SymbolInfo", client: Optional[object], model_hash: str):
        """Initialize Symbol from SymbolInfo protobuf.
        
        Args:
            pb_symbol: SymbolInfo protobuf message
            client: Optional RPC client for lazy loading children
            model_hash: Hash of the model this symbol belongs to
        """
        self._pb = pb_symbol
        self._client = client
        self._model_hash = model_hash
        self._children_cache = None
    
    @property
    def id(self) -> str:
        """Return symbol ID (fully-qualified name)."""
        return self._pb.id
    
    @property
    def name(self) -> str:
        """Return simple name."""
        return self._pb.name
    
    @property
    def kind(self) -> str:
        """Return SysML element kind."""
        return self._pb.kind
    
    @property
    def metadata(self) -> dict:
        """Return copy of symbol metadata dictionary."""
        return dict(self._pb.metadata)
    
    def children(self) -> List["Symbol"]:
        """Return all child symbols.
        
        Lazily fetches child symbols using the RPC client if available.
        Caches results after first call.
        Returns empty list if no client is set.
        
        Returns:
            List of child Symbol objects
        """
        if self._children_cache is not None:
            return self._children_cache
        
        if self._client is None:
            self._children_cache = []
            return self._children_cache
        
        result = []
        for child_id in self._pb.child_ids:
            child_info = self._client.get_symbol(self._model_hash, child_id)
            if child_info is not None:
                result.append(Symbol(child_info, self._client, self._model_hash))
        
        self._children_cache = result
        return result
    
    def attributes(self) -> List["Symbol"]:
        """Return child symbols that are attributes.
        
        Filters children to only those with 'Attribute' in their kind
        (e.g., AttributeUsage, PartAttributeUsage, AttributeDef).
        
        Returns:
            List of attribute Symbol objects
        """
        return [child for child in self.children() if "Attribute" in child.kind]
    
    def __str__(self) -> str:
        """Return human-readable string representation."""
        return f"{self.name} ({self.kind})"
    
    def __repr__(self) -> str:
        """Return developer-friendly string representation."""
        return f"Symbol(id={self.id!r}, kind={self.kind!r})"
