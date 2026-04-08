import io
import typing

import pytest

try:
    from _pytest._io.terminalwriter import TerminalWriter
except ImportError:
    try:
        from py.io import TerminalWriter
    except ImportError:
        TerminalWriter = None


@pytest.hookimpl(hookwrapper=True)
def pytest_json_runtest_stage(
    report: pytest.TestReport,
) -> typing.Generator[typing.Any, typing.Any, typing.Any]:
    outcome = yield

    stage_metadata = outcome.get_result()

    if stage_metadata and hasattr(report.longrepr, "toterminal") and TerminalWriter:
        try:
            s = io.StringIO()
            w = TerminalWriter(file=s)  # type: ignore
            w.hasmarkup = True  # Force enable ANSI color codes

            report.longrepr.toterminal(w)  # type: ignore

            colorized_output = s.getvalue()
            if colorized_output:
                stage_metadata["longrepr"] = colorized_output

        except Exception:
            pass
