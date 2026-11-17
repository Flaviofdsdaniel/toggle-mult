import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).with_name("rollback_gitops.py")
MANIFEST = Path(".github/GitOps/auth-service/deployment.yaml")


class RollbackGitOpsTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)
        self.manifest = self.root / MANIFEST
        self.manifest.parent.mkdir(parents=True)
        self.manifest.write_text(
            "apiVersion: apps/v1\nkind: Deployment\nspec:\n  template:\n"
            "    spec:\n      containers:\n        - name: auth-service\n"
            "          image: registry/auth-service:v1\n",
            encoding="utf-8",
        )
        subprocess.run(["git", "init", "-q"], cwd=self.root, check=True)
        subprocess.run(["git", "config", "user.name", "test"], cwd=self.root, check=True)
        subprocess.run(
            ["git", "config", "user.email", "test@example.invalid"],
            cwd=self.root,
            check=True,
        )
        self._commit("stable v1")

    def tearDown(self):
        self.temp_dir.cleanup()

    def _commit(self, message):
        subprocess.run(["git", "add", str(MANIFEST)], cwd=self.root, check=True)
        subprocess.run(["git", "commit", "-qm", message], cwd=self.root, check=True)

    def _run(self, expected_image):
        return subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "--service",
                "auth-service",
                "--expected-image",
                expected_image,
                "--repo-root",
                str(self.root),
            ],
            cwd=self.root,
            text=True,
            capture_output=True,
        )

    def test_rolls_back_only_image_to_previous_distinct_revision(self):
        text = self.manifest.read_text(encoding="utf-8").replace(
            "registry/auth-service:v1", "registry/auth-service:v2"
        )
        self.manifest.write_text(text, encoding="utf-8")
        self._commit("bad release v2")

        result = self._run("registry/auth-service:v2")

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("registry/auth-service:v1", self.manifest.read_text(encoding="utf-8"))
        self.assertIn("Rollback GitOps preparado", result.stdout)

    def test_skips_stale_alert_when_manifest_has_moved_on(self):
        text = self.manifest.read_text(encoding="utf-8").replace(
            "registry/auth-service:v1", "registry/auth-service:v2"
        )
        self.manifest.write_text(text, encoding="utf-8")
        self._commit("release v2")
        text = self.manifest.read_text(encoding="utf-8").replace(
            "registry/auth-service:v2", "registry/auth-service:v3"
        )
        self.manifest.write_text(text, encoding="utf-8")
        self._commit("release v3")

        result = self._run("registry/auth-service:v2")

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("registry/auth-service:v3", self.manifest.read_text(encoding="utf-8"))
        self.assertIn("Alerta obsoleto", result.stdout)

    def test_rejects_service_outside_allowlist(self):
        result = subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "--service",
                "togglemaster-alert-demo",
                "--expected-image",
                "registry/demo:v1",
                "--repo-root",
                str(self.root),
            ],
            cwd=self.root,
            text=True,
            capture_output=True,
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("microsserviço não permitido", result.stderr)


if __name__ == "__main__":
    unittest.main()
