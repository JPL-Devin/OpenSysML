"""Conversion of protobuf Value/SlotValue messages into Python values."""

from pysysml.errors import SlotError, UnsupportedValueError


def value_to_python(pb_value, resolve_instance=None):
    """Convert a protobuf Value into a plain Python value.

    Args:
        pb_value: sysml_pb2.Value message
        resolve_instance: optional callable mapping an instance id to an object;
            when omitted, instance references are returned as their integer id.

    Returns:
        int, float, bool, str, list, None, or the resolved instance object.

    Raises:
        UnsupportedValueError: If the service reported the value as unsupported.
    """
    kind = pb_value.WhichOneof('kind')
    if kind == 'int_value':
        return pb_value.int_value
    if kind == 'real_value':
        return pb_value.real_value
    if kind == 'bool_value':
        return pb_value.bool_value
    if kind == 'string_value':
        return pb_value.string_value
    if kind == 'instance_id':
        if resolve_instance is None:
            return pb_value.instance_id
        return resolve_instance(pb_value.instance_id)
    if kind == 'sequence':
        return [value_to_python(v, resolve_instance) for v in pb_value.sequence.elements]
    if kind == 'null':
        # A non-empty null carries the reason the value could not be sent.
        if pb_value.null:
            raise UnsupportedValueError(pb_value.null)
        return None
    return None


def slot_to_python(feature_name, pb_slot, resolve_instance=None):
    """Convert a protobuf SlotValue into a plain Python value.

    Args:
        feature_name (str): Slot name, used for error reporting
        pb_slot: sysml_pb2.SlotValue message
        resolve_instance: optional callable mapping an instance id to an object

    Returns:
        The scalar value, or a list for collection slots.

    Raises:
        SlotError: If evaluation failed or the slot was never materialized.
    """
    if pb_slot.error:
        raise SlotError(feature_name, pb_slot.error)

    if pb_slot.HasField('value'):
        try:
            return value_to_python(pb_slot.value, resolve_instance)
        except UnsupportedValueError as exc:
            raise SlotError(feature_name, str(exc)) from exc

    if pb_slot.values:
        try:
            return [value_to_python(v, resolve_instance) for v in pb_slot.values]
        except UnsupportedValueError as exc:
            raise SlotError(feature_name, str(exc)) from exc

    if pb_slot.materialized:
        return []

    raise SlotError(feature_name, "slot is not materialized")
