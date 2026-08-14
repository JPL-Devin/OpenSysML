"""Model class wrapping parsed SysML model."""

from pysysml.symbol import Symbol
from pysysml.conversion import FORMAT_SYSML, FORMAT_TURTLE, format_of_path
from pysysml.diagnostic import Diagnostic


class Model:
    """Represents a parsed SysML model.
    
    Attributes:
        hash (str): Model content hash (for cache lookups)
        root (Symbol): Root symbol of the model
        diagnostics (list[Diagnostic]): Parse diagnostics (errors/warnings)
    """
    
    def __init__(self, pb_response, client, source_path=None):
        """Initialize Model from protobuf ParseFileResponse.
        
        Args:
            pb_response: sysml_pb2.ParseFileResponse protobuf message
            client: Client instance for symbol navigation
            source_path (str, optional): Path the model was loaded from
        """
        self._pb = pb_response
        self._client = client
        self._source_path = source_path
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
    def connection(self):
        """Get the connection this model was loaded over."""
        return self._client
    
    @property
    def root(self):
        """Get root symbol."""
        return self._root
    
    @property
    def diagnostics(self):
        """Get list of diagnostics."""
        return self._diagnostics
    
    @property
    def source_path(self):
        """Path this model was loaded from, or None if it was loaded inline."""
        return self._source_path

    def convert(self, to_format, tolerate_syntax_errors=False):
        """Write this model out in one of the formats Systemica writes.

        Converts the source this model was parsed from, not the file as it
        stands now, so what is written is the model that was inspected: notation
        keeps its comments and lexemes, re-indented, while Turtle carries what
        the model declares. See ``docs/RDF_INTEROP.md``.

        The service holds that source in its model cache, which is bounded, so a
        model loaded long ago and many models back may have been evicted; load it
        again, or convert its path through :meth:`Connection.convert`.

        Args:
            to_format (str): 'sysml', 'kerml', 'text', 'ttl', 'turtle' or 'rdf'
            tolerate_syntax_errors (bool): Write notation back out even when the
                parser could not read all of it

        Returns:
            Conversion: The converted model; ``str()`` of it is the text

        Raises:
            ConversionError: If the model could not be written in that format
            MissingCapabilityError: If the service cannot convert
            grpc.RpcError: If the service no longer holds this model
        """
        return self.connection.convert(
            to_format,
            model_hash=self._hash,
            # A model came from ParseFile, which reads notation and nothing else.
            from_format=FORMAT_SYSML,
            tolerate_syntax_errors=tolerate_syntax_errors,
        )

    def to_sysml(self, tolerate_syntax_errors=False):
        """Write this model out as SysML textual notation.

        Returns:
            Conversion: The notation; ``str()`` of it is the text
        """
        return self.convert(FORMAT_SYSML, tolerate_syntax_errors=tolerate_syntax_errors)

    def to_turtle(self):
        """Write this model out as an RDF graph in Turtle syntax.

        Returns:
            Conversion: The Turtle; ``str()`` of it is the text
        """
        return self.convert(FORMAT_TURTLE)

    def save(self, path, to_format=None, tolerate_syntax_errors=False):
        """Write this model to ``path``, in the format its extension names.

        Args:
            path (str): File to write, created or truncated
            to_format (str, optional): Format to write, overriding the extension
            tolerate_syntax_errors (bool): Write notation back out even when the
                parser could not read all of it

        Returns:
            Conversion: What was written

        Raises:
            ValueError: If no to_format was given and the extension names none
            ConversionError: If the model could not be written in that format
        """
        conversion = self.convert(
            to_format or format_of_path(path),
            tolerate_syntax_errors=tolerate_syntax_errors,
        )
        conversion.write(path)
        return conversion

    def find(self, name):
        """Find symbol by short name or fully-qualified name (breadth-first).

        A symbol's own ``id`` is accepted as well as its short name, so the
        identifier a symbol reports can be round-tripped back into ``find``.

        Args:
            name (str): Short name ("Vehicle") or FQN ("Demo::Vehicle")

        Returns:
            Symbol or None: First matching symbol, or None if not found
        """
        def matches(symbol):
            return symbol.name == name or symbol.id == name

        # Check root first
        if matches(self.root):
            return self.root

        # Breadth-first search
        queue = [self.root]
        while queue:
            current = queue.pop(0)

            # Check each child
            for child in current.children():
                if matches(child):
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
