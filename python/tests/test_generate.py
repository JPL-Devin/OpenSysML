"""Tests for the type mapping and emitter of pysysml.generate."""

import pytest

from pysysml.generate import (
    Definition,
    Feature,
    class_names,
    collect_definitions,
    element_type,
    feature_type,
    is_definition_kind,
    is_feature_kind,
    render_module,
)
from pysysml.typefacts import Multiplicity, Specialization, SymbolFacts, TypeFacts


def definition(fqn, kind="partDef", features=(), specializations=()):
    """Build a Definition from synthetic facts."""
    name = fqn.split("::")[-1]
    return Definition(
        SymbolFacts(id=fqn, name=name, kind=kind, specializations=tuple(specializations)),
        tuple(features),
    )


def feature(name, kind="attributeUsage", type_facts=None, multiplicity=None, owner="Demo::Vehicle"):
    """Build a Feature from synthetic facts."""
    return Feature(
        SymbolFacts(
            id=f"{owner}::{name}",
            name=name,
            kind=kind,
            type=type_facts,
            multiplicity=multiplicity,
        )
    )


class FakeSymbol:
    """A Symbol-shaped node for exercising the tree walk without a service."""

    def __init__(self, facts, children=()):
        self._facts = facts
        self._children = list(children)

    @property
    def id(self):
        return self._facts.id

    @property
    def kind(self):
        return self._facts.kind

    def facts(self):
        return self._facts

    def children(self):
        return self._children


class FakeInstance:
    """An Instance-shaped holder of slot values, for exercising generated accessors."""

    def __init__(self, slots):
        self._slots = dict(slots)

    def __contains__(self, name):
        return name in self._slots

    def __getitem__(self, name):
        return self._slots[name]


def test_is_definition_kind_covers_service_kinds():
    """Every service kind ending in Def, plus metaclass, declares a type."""
    for kind in ("partDef", "attributeDef", "itemDef", "enumDef", "portDef", "metaclass"):
        assert is_definition_kind(kind)
    for kind in ("package", "partUsage", "attributeUsage", "connectorEnd"):
        assert not is_definition_kind(kind)


def test_is_feature_kind_excludes_behavioral_usages():
    """Structural usages become properties; behavioral ones are skipped."""
    for kind in ("attributeUsage", "partUsage", "itemUsage", "portUsage"):
        assert is_feature_kind(kind)
    for kind in ("actionUsage", "stateUsage", "calcUsage", "constraintUsage", "partDef"):
        assert not is_feature_kind(kind)


@pytest.mark.parametrize(
    "primitive,expected",
    [
        ("Boolean", "bool"),
        ("String", "str"),
        ("Natural", "int"),
        ("Integer", "int"),
        ("Rational", "float"),
        ("Real", "float"),
    ],
)
def test_element_type_primitives(primitive, expected):
    """Library scalars map to their Python counterparts."""
    mapped = element_type(TypeFacts(primitive=primitive), {})
    assert mapped.annotation == expected
    assert mapped.decoder == f"_t.as_{expected}"
    assert mapped.comment == ""


def test_element_type_generated_class():
    """A usage typed by a generated definition maps to that class."""
    mapped = element_type(TypeFacts(declared="Engine", resolved_id="Demo::Engine"), {"Demo::Engine": "Engine"})
    assert mapped.annotation == "Engine"
    assert mapped.decoder == "_t.as_typed(Engine)"


def test_element_type_scalar_definition_maps_to_its_scalar():
    """A definition that reduces to a library scalar holds that scalar, not an instance."""
    mapped = element_type(
        TypeFacts(declared="Celsius", resolved_id="Demo::Celsius", primitive="Real"),
        {"Demo::Celsius": "Celsius"},
    )
    assert mapped.annotation == "float"
    assert mapped.decoder == "_t.as_float"


def test_element_type_unmapped_primitive_is_object():
    """Complex and Number have no sound Python type and say so."""
    mapped = element_type(TypeFacts(primitive="Complex"), {})
    assert mapped.annotation == "object"
    assert "Complex" in mapped.comment


def test_element_type_quantity_is_object_with_unit():
    """A quantity maps to object and names its unit."""
    mapped = element_type(TypeFacts(primitive="Real", quantity=True, unit="kg"), {})
    assert mapped.annotation == "object"
    assert "kg" in mapped.comment


def test_element_type_unresolved_and_untyped():
    """An unresolved or absent type maps to object, naming what was written."""
    unresolved = element_type(TypeFacts(declared="Missing"), {})
    assert unresolved.annotation == "object"
    assert "Missing" in unresolved.comment

    untyped = element_type(None, {})
    assert untyped.annotation == "object"
    assert untyped.comment


def test_element_type_type_without_generated_class():
    """A type resolved outside the generated set maps to object, naming the FQN."""
    mapped = element_type(TypeFacts(declared="Anything", resolved_id="Base::Anything"), {})
    assert mapped.annotation == "object"
    assert "Base::Anything" in mapped.comment


@pytest.mark.parametrize(
    "multiplicity,expected",
    [
        (None, "float"),
        (Multiplicity(lower="1", upper="1"), "float"),
        (Multiplicity(lower="0", upper="1"), "float | None"),
        (Multiplicity(lower="0", upper="*"), "list[float]"),
        (Multiplicity(lower="2", upper="4"), "list[float]"),
        (Multiplicity(lower="1", upper=""), "float"),
    ],
)
def test_feature_type_multiplicity(multiplicity, expected):
    """Multiplicity decides between a bare value, an option and a list."""
    mapped = feature_type(feature("mass", type_facts=TypeFacts(primitive="Real"), multiplicity=multiplicity), {})
    assert mapped.annotation == expected


def test_feature_named_like_a_typed_object_member_is_renamed():
    """A feature named `instance` must not shadow the accessor machinery it uses."""
    source = render_module(
        [
            definition(
                "Demo::Vehicle",
                features=[feature("instance", type_facts=TypeFacts(primitive="Real"))],
            )
        ]
    )
    assert "def instance_(self) -> float:" in source
    assert '_t.slot(self, "instance", _t.as_float)' in source

    namespace: dict = {}
    exec(compile(source, "generated", "exec"), namespace)
    assert namespace["Vehicle"].instance is not None


def test_unrestricted_name_with_quotes_is_escaped():
    """A SysML unrestricted name may carry quotes and backslashes; output must import."""
    source = render_module(
        [
            definition(
                'Demo::say "hi"\\x',
                features=[
                    feature(
                        'mass "kg"\\x',
                        type_facts=TypeFacts(primitive="Real"),
                        owner='Demo::say "hi"\\x',
                    )
                ],
            )
        ]
    )
    namespace: dict = {}
    exec(compile(source, "generated", "exec"), namespace)
    generated = namespace["say__hi__x"]
    assert generated.sysml_id == 'Demo::say "hi"\\x'
    assert generated.from_instance(FakeInstance({'mass "kg"\\x': 1500.0})).mass__kg__x == 1500.0


def test_class_names_disambiguate_collisions():
    """Two definitions of the same simple name both get qualified class names."""
    definitions = [definition("A::Thing"), definition("B::Thing"), definition("A::Other")]
    names = class_names(definitions)
    assert names == {"A::Thing": "A_Thing", "B::Thing": "B_Thing", "A::Other": "Other"}


def test_class_names_sanitize_identifiers():
    """A SysML name that is not a Python identifier is made into one."""
    names = class_names([definition("P::my part"), definition("P::class")])
    assert names["P::my part"] == "my_part"
    assert names["P::class"] == "class_"


def test_collect_definitions_walks_tree_and_sorts():
    """Definitions come back sorted by FQN with their structural features."""
    engine = FakeSymbol(SymbolFacts(id="Demo::Engine", name="Engine", kind="partDef"))
    mass = FakeSymbol(SymbolFacts(id="Demo::Vehicle::mass", name="mass", kind="attributeUsage"))
    constraint = FakeSymbol(SymbolFacts(id="Demo::Vehicle::ok", name="ok", kind="constraintUsage"))
    vehicle = FakeSymbol(
        SymbolFacts(id="Demo::Vehicle", name="Vehicle", kind="partDef"), [mass, constraint]
    )
    root = FakeSymbol(SymbolFacts(id="Demo", name="Demo", kind="package"), [vehicle, engine])

    definitions = collect_definitions(root)
    assert [d.id for d in definitions] == ["Demo::Engine", "Demo::Vehicle"]
    assert [f.name for f in definitions[1].features] == ["mass"]


def test_render_module_is_importable_and_deterministic(tmp_path):
    """The emitted module imports and rendering the same input twice is identical."""
    definitions = [
        definition("Demo::Engine", features=[feature("power", type_facts=TypeFacts(primitive="Real"), owner="Demo::Engine")]),
        definition(
            "Demo::Vehicle",
            features=[
                feature("engine", kind="partUsage", type_facts=TypeFacts(declared="Engine", resolved_id="Demo::Engine")),
                feature("mass", type_facts=TypeFacts(primitive="Real")),
            ],
        ),
    ]
    source = render_module(definitions)
    assert source == render_module(definitions)

    module_path = tmp_path / "demo_types.py"
    module_path.write_text(source)
    namespace: dict = {}
    exec(compile(source, str(module_path), "exec"), namespace)
    assert namespace["Vehicle"].sysml_id == "Demo::Vehicle"
    assert issubclass(namespace["Engine"], __import__("pysysml.typed", fromlist=["typed"]).TypedObject)


def test_render_module_emits_bases_before_subclasses():
    """A specialized definition is emitted after the definition it specializes."""
    car = definition("Demo::Car", specializations=[Specialization(kind="specializes", declared="Vehicle", target_id="Demo::Vehicle")])
    vehicle = definition("Demo::Vehicle")
    source = render_module([car, vehicle])
    assert source.index("class Vehicle(") < source.index("class Car(Vehicle)")


def test_render_module_notes_ungenerated_base():
    """A specialization outside the model is reported in a comment, not silently dropped."""
    car = definition(
        "Demo::Car",
        specializations=[Specialization(kind="specializes", declared="Base", target_id="Lib::Base")],
    )
    source = render_module([car])
    assert "class Car(_t.TypedObject):" in source
    assert "# specializes Lib::Base, which has no generated class" in source


def test_render_module_uses_multiplicity_accessors():
    """List and optional features use the matching runtime accessor."""
    source = render_module(
        [
            definition(
                "Demo::Vehicle",
                features=[
                    feature("spare", type_facts=TypeFacts(primitive="Real"), multiplicity=Multiplicity("0", "1")),
                    feature("wheels", type_facts=TypeFacts(primitive="Real"), multiplicity=Multiplicity("0", "*")),
                ],
            )
        ]
    )
    assert '_t.optional_slot(self, "spare", _t.as_float)' in source
    assert '_t.list_slot(self, "wheels", _t.as_float)' in source


def test_render_module_empty_model():
    """A model with no definitions still emits an importable module."""
    source = render_module([])
    namespace: dict = {}
    exec(compile(source, "empty_types.py", "exec"), namespace)
    assert "TypedObject" not in namespace
