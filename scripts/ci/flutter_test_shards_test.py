import importlib.util
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path

spec = importlib.util.spec_from_file_location("shards", Path(__file__).with_name("flutter_test_shards.py"))
shards = importlib.util.module_from_spec(spec)
spec.loader.exec_module(shards)

class FlutterTestShardsTest(unittest.TestCase):
    def test_discovery_includes_generated_nested_and_excludes_directory_links(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "nested").mkdir()
            (root / "nested" / "generated_test.dart").touch()
            (root / "nested" / "helper.dart").touch()
            (root / "root_test.dart").touch()
            try:
                (root / "linked").symlink_to(root / "nested", target_is_directory=True)
            except OSError:
                pass
            self.assertEqual(shards.discover(root), ["nested/generated_test.dart", "root_test.dart"])

    def test_partition_is_complete_unique_and_deterministic(self):
        files = [f"file_{i}_test.dart" for i in range(25)]
        weights = {files[0]: 100, files[1]: 75}
        partitions = shards.partition(files, weights, 4)
        self.assertEqual(sorted(sum(partitions, [])), sorted(files))
        self.assertEqual(len(sum(partitions, [])), len(set(sum(partitions, []))))
        self.assertEqual(partitions, shards.partition(list(reversed(files)), weights, 4))
        self.assertNotEqual(next(i for i, p in enumerate(partitions) if files[0] in p),
                            next(i for i, p in enumerate(partitions) if files[1] in p))

    def test_plan_rejects_source_and_identity_drift(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            names = [f"{i}_test.dart" for i in range(4)]
            for name in names:
                (root / name).write_text(name)
            identity = {"sha": "same"}
            plan = {"identity": identity, "files": shards.snapshot(root),
                    "groups": [[name] for name in names]}
            with patch.object(shards, "identity", return_value=identity):
                shards.validate_plan(plan, root)
                (root / names[0]).write_text("changed")
                with self.assertRaises(ValueError):
                    shards.validate_plan(plan, root)
            with patch.object(shards, "identity", return_value={"sha": "different"}):
                with self.assertRaises(ValueError):
                    shards.validate_plan(plan, root)

    def test_missing_duplicate_or_wrong_receipts_fail(self):
        plan = {"identity": {"sha": "abc"}, "groups": [["a"], ["b"], ["c"], ["d"]]}
        receipts = [{"index": i, "identity": plan["identity"], "files": group}
                    for i, group in enumerate(plan["groups"])]
        shards.verify_receipts(plan, receipts)
        for bad in (receipts[:-1], receipts + receipts[:1],
                    [receipts[0], receipts[0], receipts[2], receipts[3]],
                    [dict(receipts[0], files=["x"])] + receipts[1:]):
            with self.assertRaises(ValueError):
                shards.verify_receipts(plan, bad)

    def test_invalid_inputs_fail_closed(self):
        for files, count in [([], 4), (["x", "x"], 1), (["x"], 0), (["x"], 2)]:
            with self.subTest(files=files, count=count), self.assertRaises(ValueError):
                shards.partition(files, {}, count)
        with self.assertRaises(ValueError):
            shards.partition(["x"], {"x": -1}, 1)

if __name__ == "__main__":
    unittest.main()
