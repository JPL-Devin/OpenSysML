"""Verdicts of the verification the service performs.

A verdict is an answer about the model: whether a constraint holds, whether a
requirement is satisfied, whether a satisfaction assertion holds. A condition
that evaluated to false is such an answer and is *not* an exception — it is what
was asked. A failure to evaluate at all (an unbound feature, incommensurable
units, an exhausted step budget) is not an answer, and is reported as
:attr:`Verdict.error`, or raised by :meth:`Verdict.raise_for_error`.
"""

from pysysml.errors import ExecutionError

#: Verdict kinds, as the service reports them.
KIND_CONSTRAINT = "constraint"
KIND_REQUIREMENT = "requirement"
KIND_SATISFY = "satisfy"


class Verdict:
    """One verification's answer.

    Truthy when the condition holds, so a verdict reads as the answer it is::

        if not model.verify_requirement("Demo::Range"):
            print(verdict.explain())

    Attributes:
        kind (str): What was verified: 'constraint', 'requirement' or 'satisfy'
        element_id (str): FQN of the element verified; empty for an anonymous
            satisfaction assertion
        element (str): The element as a reader names it — its FQN, or the
            assertion as written ("satisfy Range by cruise")
        holds (bool): Whether the condition holds
        condition (str): The condition that evaluated to false, as written, when
            the runtime names one
        instance_id (int): Instance the verdict is about, 0 when it is about
            declared values alone
        instance_type_id (str): FQN of that instance's type
        error (str): Set when evaluation failed rather than the model answering
            false; ``holds`` is then False but is no verdict
        instances (list[Instance]): The objects the call reported: the one this
            verdict is about (``instance_id``) and those reachable from it. A
            call answering several assertions reports one graph for them all, so
            filter on ``instance_id`` to single out this verdict's own object
        diagnostics (list[Diagnostic]): Diagnostics the service reported
    """

    def __init__(self, pb_verdict, instances=None, diagnostics=None):
        self._pb = pb_verdict
        self.instances = list(instances or [])
        self.diagnostics = list(diagnostics or [])

    @property
    def kind(self):
        """What was verified."""
        return self._pb.kind

    @property
    def element_id(self):
        """FQN of the element verified."""
        return self._pb.element_id

    @property
    def element(self):
        """The element as a reader names it."""
        return self._pb.element or self._pb.element_id

    @property
    def holds(self):
        """Whether the condition holds."""
        return self._pb.holds

    @property
    def condition(self):
        """The condition that evaluated to false, as written, when named."""
        return self._pb.condition

    @property
    def instance_id(self):
        """Instance the verdict is about, 0 for none."""
        return self._pb.instance_id

    @property
    def instance_type_id(self):
        """FQN of the type of the instance the verdict is about."""
        return self._pb.instance_type_id

    @property
    def error(self):
        """Why evaluation failed, when it failed rather than answering."""
        return self._pb.error

    @property
    def evaluated(self):
        """Whether ``holds`` is an answer about the model at all."""
        return not self._pb.error

    def raise_for_error(self):
        """Raise :class:`~pysysml.errors.ExecutionError` if evaluation failed.

        A verdict of false raises nothing: it is the model's answer. Call this
        where a failure to evaluate must not be read as a failing verdict.

        Returns:
            Verdict: self, so a call can be chained

        Raises:
            ExecutionError: If the condition could not be evaluated
        """
        if self._pb.error:
            raise ExecutionError(
                f"{self._named()}: {self._pb.error}",
                diagnostics=self.diagnostics,
            )
        return self

    def _named(self):
        """How a line about this verdict names what it is about.

        An assertion's text already says the kind it is ("satisfy r by p", "not
        satisfy r by p"), so the kind is not repeated in front of it.
        """
        element = self.element
        if self.kind in element.split():
            return element
        return f"{self.kind} {element}"

    def explain(self):
        """One line saying what the verdict is and why."""
        subject = ""
        if self.instance_id:
            subject = f" (on {self.instance_type_id or 'instance'} ID: {self.instance_id})"
        if self._pb.error:
            return f"? {self._named()}{subject}: {self._pb.error}"
        if self.holds:
            return f"\u2713 {self._named()} holds{subject}"
        detail = (
            f": condition evaluated to false: {self.condition}"
            if self.condition
            else ": condition evaluated to false"
        )
        return f"\u2717 {self._named()} fails{subject}{detail}"

    def __bool__(self):
        """A verdict is truthy when the condition holds."""
        return bool(self._pb.holds)

    def __str__(self):
        return self.explain()

    def __repr__(self):
        return (
            f"Verdict(kind={self.kind!r}, element={self.element!r}, "
            f"holds={self.holds!r}, condition={self.condition!r}, "
            f"error={self.error!r})"
        )


class CalcResult:
    """What a calculation computed.

    A calculation invoked with arguments returns one value, which is
    :attr:`value`. A calc usage evaluated from its own members computes its
    output features instead, which are :attr:`outputs` — a mapping of feature
    name to value, in declaration order (SysML 7.17).

    Attributes:
        value: The value an invocation returned, or None when outputs carry the
            answer
        outputs (dict): Output features a calc usage computed
        diagnostics (list[Diagnostic]): Diagnostics the service reported
    """

    def __init__(self, value, outputs, diagnostics=None):
        self.value = value
        self.outputs = dict(outputs or {})
        self.diagnostics = list(diagnostics or [])

    def __str__(self):
        if self.outputs:
            return ", ".join(f"{name} = {val}" for name, val in self.outputs.items())
        return str(self.value)

    def __repr__(self):
        return f"CalcResult(value={self.value!r}, outputs={self.outputs!r})"
