"""Instance class wrapping runtime-materialized objects."""


class Instance:
    """Represents a runtime instance of a part/usage.
    
    Attributes:
        id (int): Unique instance identifier
        type_symbol_id (str): FQN of the def/usage this instantiates
        slots (dict): Feature name → SlotValue
    """
    
    def __init__(self, pb_instance):
        """Initialize from protobuf Instance message.
        
        Args:
            pb_instance: sysml_pb2.Instance protobuf message
        """
        self._pb = pb_instance
    
    @property
    def id(self):
        """Get instance ID."""
        return self._pb.id
    
    @property
    def type_symbol_id(self):
        """Get type symbol FQN."""
        return self._pb.type_symbol_id
    
    @property
    def slots(self):
        """Get all slots as dict {feature_name: SlotValue}."""
        return dict(self._pb.slots)
    
    def get_slot(self, feature_name):
        """Get slot value for a feature.
        
        Args:
            feature_name (str): Name of feature
            
        Returns:
            SlotValue or None if not found
        """
        return self._pb.slots.get(feature_name)
    
    def __str__(self):
        return f"Instance(id={self.id}, type={self.type_symbol_id})"
    
    def __repr__(self):
        return f"Instance(id={self.id}, type={self.type_symbol_id!r}, slots={len(self.slots)})"
