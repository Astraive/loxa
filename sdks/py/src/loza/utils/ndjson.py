import json


def encode_line(payload):
    return json.dumps(payload, separators=(",", ":")) + "\n"


def decode_lines(text: str):
    return [json.loads(line) for line in text.splitlines() if line.strip()]
