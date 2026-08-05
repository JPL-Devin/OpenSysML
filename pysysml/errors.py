"""Runtime error exceptions."""


class RuntimeError(Exception):
    """Raised when runtime operation (eval/instantiate/execute) fails.
    
    Attributes:
        message (str): Error description
        diagnostics (list): List of Diagnostic objects (if available)
    """
    
    def __init__(self, message, diagnostics=None):
        super().__init__(message)
        self.message = message
        self.diagnostics = diagnostics or []
