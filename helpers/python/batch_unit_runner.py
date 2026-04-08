import json
import os
import re
import resource
import selectors
import signal
import time
import typing

import pytest

CLOCK_TICKS = os.sysconf("SC_CLK_TCK")
PAGE_SIZE = os.sysconf("SC_PAGESIZE")

STATUS_POLL_TIMEOUT = 0.1  # seconds
TIMEOUT_CHECK_INTERVAL = 0.1  # seconds
STDIO_REDIRECT_ERROR_EXIT_CODE = 2


class TimeLimit(typing.TypedDict):
    real: float
    wall: float


class Limit(typing.TypedDict):
    processes: int
    time: TimeLimit
    memory: int
    output_size: int
    open_files: int


class Case(typing.TypedDict):
    name: str
    output: str
    metadata: str
    limit: Limit


class Config(typing.TypedDict):
    submission: str
    cases: list[Case]


class Execution:
    idx: int
    pid: int
    fd: int
    start_time: float

    def __init__(self, idx: int, pid: int, fd: int, start_time: float) -> None:
        self.idx = idx
        self.pid = pid
        self.fd = fd
        self.start_time = start_time


class ExecutionResult:
    output: str
    time: float
    exit_code: int
    exit_signal: int

    def __init__(self, output: str, time: float, exit_code: int, exit_signal: int) -> None:
        self.output = output
        self.time = time
        self.exit_code = exit_code
        self.exit_signal = exit_signal


def get_proc_time(pid: int) -> float:
    with open(f"/proc/{pid}/stat", "r", encoding="utf-8") as f:
        raw = f.read()

    match = re.match(r"^(-?[0-9]+) \((.*?)\) (.) ([0-9- ]*)$", raw)

    if not match:
        raise ValueError("Failed to parse /proc/pid/stat")

    rest = [int(val) for val in match[4].split()]

    if len(rest) != 49:
        raise ValueError("Failed to parse /proc/pid/stat")

    utime = int(rest[10])
    stime = int(rest[11])

    return (utime + stime) / CLOCK_TICKS


def generate_metadata_file(
    case: Case,
    execution: Execution,
    killed: bool,
    exit_status: int,
    resource_usage: resource.struct_rusage,
) -> None:
    exit_code = exit_status >> 8
    exit_signal = exit_status & 0b01111111

    with open(case["metadata"], "w", encoding="utf-8") as f:
        if exit_code == STDIO_REDIRECT_ERROR_EXIT_CODE:
            f.write("status:XX\n")
            f.write("message:Failed to redirect STDIO\n")
        else:
            f.write(f"time:{resource_usage.ru_utime + resource_usage.ru_stime:.3f}\n")
            f.write(f"time-wall:{time.monotonic() - execution.start_time:.3f}\n")
            f.write(f"max-rss:{resource_usage.ru_maxrss}\n")
            f.write(f"csw-voluntary:{resource_usage.ru_nvcsw}\n")
            f.write(f"csw-forced:{resource_usage.ru_nivcsw}\n")
            if killed:
                if exit_signal != 0:
                    f.write(f"exitsig:{exit_signal}\n")
                elif killed:
                    f.write(f"exitcode:{exit_code}\n")
                f.write("killed:1\n")
                f.write("status:TO\n")
                f.write("message:Timed out\n")
            elif exit_signal != 0:
                f.write(f"exitsig:{exit_signal}\n")
                f.write("status:SG\n")
                f.write(f"message:Caught fatal signal {exit_signal}\n")
            else:
                f.write(f"exitcode:{exit_code}\n")
                if exit_code != 0:
                    f.write("status:RE\n")
                    f.write(f"message:Exitted with status code {exit_code}\n")


def main() -> None:
    with open("config.json", "rb") as f:
        config: Config = json.load(f)

    cases = config["cases"]

    executions: dict[int, Execution] = {}
    selector = selectors.DefaultSelector()

    for idx, case in enumerate(cases):
        pid = os.fork()

        if pid == 0:  # Child process
            resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
            resource.setrlimit(resource.RLIMIT_MEMLOCK, (0, 0))
            resource.setrlimit(
                resource.RLIMIT_NPROC, (case["limit"]["processes"], case["limit"]["processes"])
            )
            resource.setrlimit(
                resource.RLIMIT_AS, (case["limit"]["memory"], case["limit"]["memory"])
            )
            resource.setrlimit(
                resource.RLIMIT_FSIZE, (case["limit"]["output_size"], case["limit"]["output_size"])
            )
            resource.setrlimit(
                resource.RLIMIT_NOFILE, (case["limit"]["open_files"], case["limit"]["open_files"])
            )

            os.close(1)
            if os.open(case["output"], os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o666) != 1:
                os._exit(STDIO_REDIRECT_ERROR_EXIT_CODE)
            if os.dup2(1, 2) < 0:
                os._exit(STDIO_REDIRECT_ERROR_EXIT_CODE)

            os._exit(
                pytest.main(
                    [
                        "-p",
                        "no:cacheprovider",
                        f"submission_test.py::{case['name']}",
                    ]
                )
            )

        executions[idx] = Execution(idx, pid, -1, time.monotonic())

    for idx, execution in executions.items():
        execution.fd = os.pidfd_open(execution.pid, 0)
        selector.register(execution.fd, selectors.EVENT_READ, execution)

    last_timeout_check = 0

    while executions:
        events = selector.select(timeout=STATUS_POLL_TIMEOUT)
        for key, _ in events:
            execution: Execution = key.data
            _, exit_status, resource_usage = os.wait4(execution.pid, 0)

            generate_metadata_file(
                cases[execution.idx], execution, False, exit_status, resource_usage
            )

            selector.unregister(execution.fd)
            del executions[execution.idx]

        current_time = time.monotonic()

        if current_time - last_timeout_check > TIMEOUT_CHECK_INTERVAL:
            last_timeout_check = current_time
            for idx, execution in executions.copy().items():
                case = cases[execution.idx]

                if (
                    get_proc_time(execution.pid) > case["limit"]["time"]["real"]
                    or current_time - execution.start_time > case["limit"]["time"]["wall"]
                ):
                    selector.unregister(execution.fd)
                    os.kill(execution.pid, signal.SIGKILL)
                    _, exit_status, resource_usage = os.wait4(execution.pid, 0)

                    generate_metadata_file(
                        cases[execution.idx], execution, True, exit_status, resource_usage
                    )

                    del executions[execution.idx]


if __name__ == "__main__":
    main()
