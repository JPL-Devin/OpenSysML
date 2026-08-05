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
    
    def _repr_html_(self):
        """IPython rich display: slots table."""
        html = ['<div style="font-family: monospace; padding: 10px; border: 1px solid #ddd;">']
        html.append(f'<h4 style="margin-top: 0;">Instance #{self.id}</h4>')
        html.append(f'<p><strong>Type:</strong> <code>{self.type_symbol_id}</code></p>')
        
        if self.slots:
            html.append('<table style="border-collapse: collapse; width: 100%; margin-top: 10px;">')
            html.append('<thead><tr style="background: #f0f0f0;">')
            html.append('<th style="border: 1px solid #ccc; padding: 5px; text-align: left;">Feature</th>')
            html.append('<th style="border: 1px solid #ccc; padding: 5px; text-align: left;">Value</th>')
            html.append('<th style="border: 1px solid #ccc; padding: 5px; text-align: left;">Materialized</th>')
            html.append('</tr></thead><tbody>')
            
            for feature_name, slot_value in self.slots.items():
                html.append('<tr>')
                html.append(f'<td style="border: 1px solid #ccc; padding: 5px;"><code>{feature_name}</code></td>')
                
                # Format value
                if slot_value.HasField('value'):
                    value_str = self._format_value(slot_value.value)
                elif slot_value.values:
                    value_str = '[' + ', '.join(self._format_value(v) for v in slot_value.values) + ']'
                else:
                    value_str = '<em>None</em>'
                
                html.append(f'<td style="border: 1px solid #ccc; padding: 5px;">{value_str}</td>')
                html.append(f'<td style="border: 1px solid #ccc; padding: 5px;">{"✓" if slot_value.materialized else ""}</td>')
                html.append('</tr>')
            
            html.append('</tbody></table>')
        else:
            html.append('<p><em>No slots</em></p>')
        
        html.append('</div>')
        return ''.join(html)
    
    def _format_value(self, pb_value):
        """Format a protobuf Value for HTML display."""
        kind = pb_value.WhichOneof('kind')
        if kind == 'int_value':
            return str(pb_value.int_value)
        elif kind == 'real_value':
            return str(pb_value.real_value)
        elif kind == 'bool_value':
            return str(pb_value.bool_value)
        elif kind == 'string_value':
            return f'"{pb_value.string_value}"'
        elif kind == 'instance_id':
            return f'Instance#{pb_value.instance_id}'
        elif kind == 'null':
            return 'null'
        else:
            return '<em>unknown</em>'
