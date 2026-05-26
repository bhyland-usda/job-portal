from importlib.util import module_from_spec, spec_from_file_location
from pathlib import Path
import sys


ROOT = Path(__file__).resolve().parents[1]
CMD_BOTS_TESTS = ROOT / "cmd" / "bots" / "tests"


def ensure_browser_harness():
    if "browser_harness" in sys.modules:
        return

    path = CMD_BOTS_TESTS / "browser_harness.py"
    spec = spec_from_file_location("browser_harness", path)
    if spec is None or spec.loader is None:
        raise ImportError("Unable to load browser_harness")
    module = module_from_spec(spec)
    sys.modules["browser_harness"] = module
    spec.loader.exec_module(module)


def load_module(name: str):
    ensure_browser_harness()
    path = CMD_BOTS_TESTS / f"{name}.py"
    spec = spec_from_file_location(f"cmd_bots_{name}", path)
    if spec is None or spec.loader is None:
        raise ImportError(f"Unable to load test module: {name}")
    module = module_from_spec(spec)
    spec.loader.exec_module(module)
    return module
