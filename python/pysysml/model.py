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
    
    def get(self, fqn):
        """Get symbol by fully-qualified name (e.g., "Demo::Vehicle").

        Args:
            fqn (str): Fully-qualified name to look up

        Returns:
            Symbol or None: Matching symbol, or None if not found
        """
        if self.root.id == fqn:
            return self.root

        queue = [self.root]
        while queue:
            current = queue.pop(0)
            for child in current.children():
                if child.id == fqn:
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
    
    def _repr_html_(self):
        """IPython rich display: tree view + diagnostic summary."""
        from html import escape
        
        # Count diagnostics by severity
        errors = sum(1 for d in self.diagnostics if d.severity == 'error')
        warnings = sum(1 for d in self.diagnostics if d.severity == 'warning')
        
        # Build HTML
        html = ['<div style="font-family: monospace; padding: 10px; border: 1px solid #ccc;">']
        html.append(f'<h3>Model: {escape(self.root.name)}</h3>')
        html.append(f'<p><strong>Hash:</strong> <code>{escape(self.hash[:12])}...</code></p>')
        html.append(f'<p><strong>Root Kind:</strong> {escape(self.root.kind)}</p>')
        
        # Diagnostic summary
        if self.diagnostics:
            html.append('<p><strong>Diagnostics:</strong>')
            if errors:
                html.append(f' <span style="color: red;">{errors} error(s)</span>')
            if warnings:
                html.append(f' <span style="color: orange;">{warnings} warning(s)</span>')
            html.append('</p>')
            
            # Show first 5 diagnostics
            html.append('<ul style="margin-top: 5px;">')
            for diag in self.diagnostics[:5]:
                color = 'red' if diag.severity == 'error' else 'orange'
                html.append(f'<li style="color: {color};">{escape(diag.message)} (line {diag.span.start_line})</li>')
            if len(self.diagnostics) > 5:
                html.append(f'<li>... and {len(self.diagnostics) - 5} more</li>')
            html.append('</ul>')
        else:
            html.append('<p style="color: green;"><strong>✓</strong> No diagnostics</p>')
        
        html.append('</div>')
        return ''.join(html)
