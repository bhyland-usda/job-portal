from _cmd_bots_loader import load_module


_module = load_module('test_visual_audit_sweep')
VisualAuditSweepTests = _module.VisualAuditSweepTests
load_tests = _module.load_tests
