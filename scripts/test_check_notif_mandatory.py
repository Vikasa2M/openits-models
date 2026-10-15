"""Unit tests for scripts/check-notif-mandatory.py (run by `make test-scripts`)."""
import importlib.util
import os
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("cnm", os.path.join(HERE, "check-notif-mandatory.py"))
cnm = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cnm)

TEMPLATE = """module tm {
  yang-version 1.1;
  namespace "urn:test:tm";
  prefix tm;
  identity k;
  identity kind-a { base k; }
  grouping hdr { leaf observed-by { type string; } }
  notification n {
    uses hdr;
    leaf kind { type identityref { base k; } }
    leaf last-contact { type string; }
    container nested { leaf deep { type string; } }
    %s
  }
}
"""


def errors_for(stmt):
    with tempfile.TemporaryDirectory() as d:
        path = os.path.join(d, "tm.yang")
        with open(path, "w", encoding="utf-8") as f:
            f.write(TEMPLATE % stmt)
        s = cnm.Schema()
        s.load(path)
        return s.mandatory_for_kind_errors()


class MandatoryForKind(unittest.TestCase):
    def test_direct_leaf_ok(self):
        self.assertEqual(errors_for('mandatory-for-kind "last-contact kind-a";'), [])

    def test_leaf_via_uses_ok(self):
        self.assertEqual(errors_for('mandatory-for-kind "observed-by kind-a";'), [])

    def test_unknown_leaf_fails(self):
        errs = errors_for('mandatory-for-kind "no-such-leaf kind-a";')
        self.assertTrue(any("not a child of the notification" in e for e in errs), errs)

    def test_leaf_below_container_fails(self):
        errs = errors_for('mandatory-for-kind "deep kind-a";')
        self.assertTrue(any("not a child of the notification" in e for e in errs), errs)

    def test_unknown_identity_fails(self):
        errs = errors_for('mandatory-for-kind "last-contact nope";')
        self.assertTrue(any("not defined" in e for e in errs), errs)


if __name__ == "__main__":
    unittest.main()
