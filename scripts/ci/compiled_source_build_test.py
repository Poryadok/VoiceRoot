#!/usr/bin/env python3
"""Contract for the protected staging Go image build, not runtime evidence."""
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / '.github/workflows/ci.yml'
ARG = "          build-args: ${{ contains(fromJSON('[\"user\",\"realtime\",\"notification\"]'), matrix.service) && format('VOICE_SOURCE_SHA={0}', github.sha) || '' }}"


def validate(text):
    job = text.split('  staging-images-push:\n', 1)[1].split('\n  staging-images-promote:', 1)[0]
    assert "github.event_name == 'push'" in job
    assert "github.ref == 'refs/heads/master'" in job
    build = job.split('      - name: Build and push ${{ matrix.service }} image\n', 1)[1]
    assert 'uses: docker/build-push-action@v6' in build
    assert build.count('          build-args:') == 1
    assert ARG in build.splitlines()


class CompiledSourceBuildContract(unittest.TestCase):
    def test_workflow(self):
        validate(WORKFLOW.read_text(encoding='utf-8'))

    def test_missing_wrong_and_untrusted_arguments_fail(self):
        source = WORKFLOW.read_text(encoding='utf-8')
        for replacement in ('', ARG.replace('github.sha', 'inputs.source_sha'),
                            ARG.replace('github.sha', 'github.event.pull_request.head.sha'),
                            ARG.replace('VOICE_SOURCE_SHA', 'SOURCE_SHA'),
                            ARG.replace('notification', 'messaging')):
            with self.subTest(replacement=replacement), self.assertRaises(AssertionError):
                validate(source.replace(ARG, replacement))


if __name__ == '__main__':
    unittest.main()
