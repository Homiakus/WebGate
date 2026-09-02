#!/usr/bin/env python3
"""Unit tests for the WebGate automated mutation testing engine."""

from __future__ import annotations

import unittest
from pathlib import Path

from scripts.run_mutation_tests import (
    MUTATION_SUITE,
    MutationDefinition,
    MutationResult,
    run_mutation,
)

ROOT = Path(__file__).resolve().parents[2]


class MutationRunnerTests(unittest.TestCase):
    def test_mutation_suite_definitions_valid(self):
        self.assertGreaterEqual(len(MUTATION_SUITE), 8)
        for mutant in MUTATION_SUITE:
            self.assertTrue(mutant.file_path.exists(), f"File {mutant.file_path} must exist")
            text = mutant.file_path.read_text(encoding="utf-8")
            self.assertIn(
                mutant.target_text,
                text,
                f"Target text for {mutant.name} must exist in {mutant.file_path.name}",
            )
            self.assertTrue(len(mutant.test_command) >= 2)
            self.assertTrue(mutant.cwd.exists())

    def test_nonexistent_file_returns_setup_error(self):
        fake_mutant = MutationDefinition(
            name="FAKE_MUTANT",
            description="Fake mutant on nonexistent file",
            file_path=ROOT / "nonexistent" / "file.txt",
            target_text="foo",
            replacement_text="bar",
            test_command=["go", "test"],
            cwd=ROOT,
        )
        res = run_mutation(fake_mutant)
        self.assertEqual(res.status, "SETUP_ERROR")

    def test_missing_target_text_returns_setup_error(self):
        real_file = ROOT / "scripts" / "check_architecture.py"
        fake_mutant = MutationDefinition(
            name="FAKE_TARGET_TEXT",
            description="Fake mutant with missing target text",
            file_path=real_file,
            target_text="DEFINITELY_NOT_IN_ARCHITECTURE_SCRIPT_XYZ_12345",
            replacement_text="bar",
            test_command=["python", "scripts/check_architecture.py"],
            cwd=ROOT,
        )
        res = run_mutation(fake_mutant)
        self.assertEqual(res.status, "SETUP_ERROR")


if __name__ == "__main__":
    unittest.main()
