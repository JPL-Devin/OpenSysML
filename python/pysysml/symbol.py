"""Symbol class for navigating the semantic model."""

from typing import List, Optional, TYPE_CHECKING

from pysysml.typefacts import Multiplicity, Specialization, SymbolFacts, TypeFacts

if TYPE_CHECKING:
    from pysysml.proto import sysml_pb2

# Kind strings emitted by the service (internal/core/symbols: symbolKindNames).
# Matched case-insensitively so older PascalCase producers still work.
ATTRIBUTE_KINDS = frozenset({"attributedef", "attributeusage"})
PART_KINDS = frozenset({"partdef", "partusage"})


class Symbol:
    """Represents a symbol in the SysML model (definition or usage).
    
    Wraps a SymbolInfo protobuf message and provides lazy navigation
    through the symbol tree via an optional RPC client.
    
    Attributes:
        id: Unique symbol identifier (fully-qualified name)
        name: Simple name of the symbol
        kind: SysML element kind (e.g., "partDef", "attributeUsage")
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
    
    @property
    def type_facts(self) -> Optional[TypeFacts]:
        """Return the symbol's static type, or None when it carries no type."""
        if not self._pb.HasField("type_info"):
            return None
        return TypeFacts.from_pb(self._pb.type_info)

    @property
    def multiplicity(self) -> Optional[Multiplicity]:
        """Return the declared multiplicity range, or None when undeclared."""
        if not self._pb.HasField("multiplicity"):
            return None
        return Multiplicity.from_pb(self._pb.multiplicity)

    @property
    def specializations(self) -> List[Specialization]:
        """Return all generalization edges declared on this symbol."""
        return [Specialization.from_pb(spec) for spec in self._pb.specializations]

    def facts(self) -> SymbolFacts:
        """Return this symbol's static facts, detached from the protobuf message."""
        return SymbolFacts(
            id=self.id,
            name=self.name,
            kind=self.kind,
            type=self.type_facts,
            multiplicity=self.multiplicity,
            specializations=tuple(self.specializations),
        )

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
        """Return child symbols that are attribute definitions or usages.
        
        Returns:
            List of attribute Symbol objects
        """
        return [child for child in self.children() if child.kind.lower() in ATTRIBUTE_KINDS]
    
    def parts(self) -> List["Symbol"]:
        """Return child symbols that are part definitions or usages.
        
        Returns:
            List of part Symbol objects
        """
        return [child for child in self.children() if child.kind.lower() in PART_KINDS]
    
    def get_attr(self, name: str) -> Optional["Symbol"]:
        """Get attribute by name.
        
        Args:
            name: Attribute name to search for
            
        Returns:
            Symbol if found, None otherwise
        """
        for attr in self.attributes():
            if attr.name == name:
                return attr
        return None
    
    def to_dataframe(self):
        """Convert children to pandas DataFrame.
        
        Creates a DataFrame with columns: name, kind, id.
        Requires pandas to be installed.
        
        Returns:
            pandas.DataFrame with child symbols
            
        Raises:
            ImportError: If pandas is not installed
        """
        try:
            import pandas as pd
        except ImportError:
            raise ImportError(
                "pandas is required for to_dataframe(). "
                "Install with: pip install pandas"
            )
        
        children = self.children()
        if not children:
            return pd.DataFrame(columns=['name', 'kind', 'id'])
        
        data = {
            'name': [child.name for child in children],
            'kind': [child.kind for child in children],
            'id': [child.id for child in children],
        }
        
        return pd.DataFrame(data)
    
    def __str__(self) -> str:
        """Return human-readable string representation."""
        return f"{self.name} ({self.kind})"
    
    def __repr__(self) -> str:
        """Return developer-friendly string representation."""
        return f"Symbol(id={self.id!r}, kind={self.kind!r})"
    
    def _repr_html_(self) -> str:
        """IPython rich display: formatted definition."""
        from html import escape
        
        html = ['<div style="font-family: monospace; padding: 10px; border: 1px solid #ddd; background: #f9f9f9;">']
        html.append(f'<h4 style="margin-top: 0;">{escape(self.name)}</h4>')
        html.append(f'<p><strong>Kind:</strong> <code>{escape(self.kind)}</code></p>')
        html.append(f'<p><strong>ID:</strong> <code>{escape(self.id)}</code></p>')
        
        # Show metadata if present
        if self.metadata:
            html.append('<p><strong>Metadata:</strong></p>')
            html.append('<ul style="margin: 5px 0;">')
            for key, value in self.metadata.items():
                html.append(f'<li><code>{escape(key)}</code>: {escape(str(value))}</li>')
            html.append('</ul>')
        
        # Show children summary
        children = self.children()
        if children:
            html.append(f'<p><strong>Children:</strong> {len(children)} symbol(s)</p>')
            
            # Group by kind
            by_kind = {}
            for child in children:
                by_kind.setdefault(child.kind, []).append(child)
            
            html.append('<ul style="margin: 5px 0;">')
            for kind, items in sorted(by_kind.items()):
                names = ', '.join(escape(item.name) for item in items[:5])
                if len(items) > 5:
                    names += f', ... (+{len(items)-5} more)'
                html.append(f'<li>{escape(kind)}: {names}</li>')
            html.append('</ul>')
        else:
            html.append('<p><em>No children</em></p>')
        
        html.append('</div>')
        return ''.join(html)
