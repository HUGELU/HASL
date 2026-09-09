"""Contract tests requiring no model weights; real GPU training is a separate integration gate."""
import tempfile
from pathlib import Path
import unittest
import local_models as worker


class WorkerContracts(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.addCleanup(self.temp.cleanup)

    def model_job(self):
        model = self.root / "model"
        model.mkdir()
        (model / "model_index.json").write_text("{}")
        return {"kind": "train_lora", "steps": 20, "output_dir": str(self.root), "image_model": str(model)}

    def test_diagnostics_needs_no_model(self):
        self.assertEqual(worker.validate_job({"kind": "diagnostics", "steps": 1, "output_dir": str(self.root)}), self.root)

    def test_rejects_download_identifier(self):
        job = self.model_job()
        job["image_model"] = "org/not-a-local-model"
        with self.assertRaisesRegex(ValueError, "local"):
            worker.validate_job(job)

    def test_step_budget(self):
        for steps in [0, 2001, True, 4.2]:
            with self.assertRaises(ValueError):
                worker.validate_job({"kind": "diagnostics", "steps": steps, "output_dir": str(self.root)})

    def test_prevents_training_validation_leakage(self):
        job = self.model_job()
        rows = []
        for i in range(4):
            p = self.root / f"{i}.png"
            p.write_bytes(b"contract fixture")
            rows.append({"asset": str(i), "path": str(p), "caption": "A supplied image", "group": str(i)})
        job.update(train=rows[:3], validation=rows[3:])
        self.assertEqual(worker.validate_job(job), self.root)
        job["validation"][0]["group"] = "1"
        with self.assertRaisesRegex(ValueError, "independent"):
            worker.validate_job(job)

    def test_requires_captions(self):
        job = self.model_job()
        job.update(train=[], validation=[])
        with self.assertRaises(ValueError):
            worker.validate_job(job)


if __name__ == "__main__":
    unittest.main()
