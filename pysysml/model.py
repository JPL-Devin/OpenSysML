"""Model class wrapping parsed SysML model."""

from pysysml.symbol import Symbol
from pysysml.diagnostic import Diagnostic


class Model:
    """Represents a parsed SysML model.
    
    Attributes:
        hash (str): Model content hash (for cache lookups)
        root (Symbol): Root symbol of the model
        diagnostics (list[Diagnostic]): Parse diagnostics (errors/warnings)
    """
    
    def __init__(self, pb_response, client):
        """Initialize Model from protobuf ParseFileResponse.
        
        Args:
            pb_response: sysml_pb2.ParseFileResponse protobuf message
            client: Client instance for symbol navigation
        """
        self._pb = pb_response
        self._client = client
        self._hash = pb_response.model_hash
        self._root = Symbol(pb_response.root, client, self._hash)
        self._diagnostics = [
            Diagnostic(pb_diag) for pb_diag in pb_response.diagnostics
        ]
    
    @property
    def hash(self):
        """Get model content hash."""
        return self._hash
    
    @property
    def root(self):
        """Get root symbol."""
        return self._root
    
    @property
    def diagnostics(self):
        """Get list of diagnostics."""
        return self._diagnostics
    
    def find(self, name):
        """Find symbol by short name (breadth-first search).
        
        Searches the symbol tree starting from root, returning the first
        symbol whose name matches. Returns None if not found.
        
        Args:
            name (str): Short name to search for (e.g., "Vehicle")
        
        Returns:
            Symbol or None: First matching symbol, or None if not found
        """
        # Check root first
        if self.root.name == name:
            return self.root
        
        # Breadth-first search
        queue = [self.root]
        while queue:
            current = queue.pop(0)
            
            # Check each child
            for child in current.children():
                if child.name == name:
                    return child
                queue.append(child)
        
        return None
    
    def __str__(self):
        """String representation: 'Model: name (kind)'."""
        return f"Model: {self.root.name} ({self.root.kind})"
    
    def __repr__(self):
        """Detailed representation."""
        diag_count = len(self.diagnostics)
        return (
            f"Model(hash={self.hash!r}, root={self.root.name!r}, "
            f"diagnostics={diag_count})"
        )
