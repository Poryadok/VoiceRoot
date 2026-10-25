"""Check canonical Space RPC coverage in each independently consumed Go SDK."""
import pathlib
import re
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[2]


def block(source, declaration):
    match = re.search(declaration + r"\s*\{(.*?)\n\}", source, re.S)
    if match is None:
        raise AssertionError("missing declaration: " + declaration)
    return match.group(1)


class SpaceRPCClosure(unittest.TestCase):
    def test_each_go_client_server_and_descriptor_covers_canonical_rpc_set(self):
        canonical = (ROOT / "protos/voice/space/v1/space.proto").read_text("utf-8")
        expected = set(re.findall(r"\brpc\s+(\w+)\s*\(", block(canonical, r"service SpaceService")))
        self.assertTrue(expected, "canonical service must contain RPCs")
        for service in ("voice", "user"):
            with self.subTest(sdk=service):
                path = ROOT / f"src/backend/{service}/pb/voice/space/v1/space_grpc.pb.go"
                source = path.read_text("utf-8")
                for kind in ("Client", "Server"):
                    methods = set(re.findall(
                        r"^\s*(\w+)\((?:ctx )?context\.Context\b",
                        block(source, rf"type SpaceService{kind} interface"), re.M))
                    self.assertEqual(expected, methods, f"{service} {kind} differs from canonical RPCs")
                descriptor = block(source, r"var SpaceService_ServiceDesc = grpc\.ServiceDesc")
                self.assertEqual(expected, set(re.findall(r'MethodName:\s*"(\w+)"', descriptor)),
                                 f"{service} ServiceDesc differs from canonical RPCs")
                self.assertEqual(expected, set(re.findall(r"SpaceService_(\w+)_FullMethodName\s*=", source)),
                                 f"{service} full method constants differ from canonical RPCs")


if __name__ == "__main__":
    unittest.main()
