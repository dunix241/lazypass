"""Validate the config schema and representative editor inputs."""

import json
from pathlib import Path
import unittest

from jsonschema import Draft7Validator


class ConfigSchemaTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        schema = json.loads(Path(__file__).with_name("config.schema.json").read_text())
        Draft7Validator.check_schema(schema)
        cls.validator = Draft7Validator(schema)

    def test_valid_configs(self):
        configs = [
            {},
            *({"updates": {"mode": mode}} for mode in ("off", "notify", "auto")),
            {"updates": {"mode": "auto", "cacheMaxAge": "15m"}},
            {"length": 256, "theme": "my-custom-theme", "vaults": None},
            {"upper": False, "lower": False, "numbers": False, "symbols": True},
            {
                "vaults": [
                    {"name": "personal", "provider": "age", "identityFile": "/identity"}
                ]
            },
        ]
        for config in configs:
            with self.subTest(config=config):
                self.validator.validate(config)

    def test_invalid_configs(self):
        configs = [
            {"vault": {"provider": "pass"}},
            {"updates": "automatic"},
            {"updates": {"mode": "auto", "cacheMaxAge": "tomorrow"}},
            {"updates": True},
            {"lenght": 20},
            {"length": 3},
            {"length": 257},
            {"length": "20"},
            {"upper": False, "lower": False, "numbers": False},
            {"vaults": [{"provider": "unknown"}]},
            {"vaults": [{"name": "Bad Name", "provider": "age"}]},
            {"vaults": [{"provider": "pass", "storeDirectory": "/vault"}]},
        ]
        for config in configs:
            with self.subTest(config=config):
                self.assertFalse(self.validator.is_valid(config))


if __name__ == "__main__":
    unittest.main()
