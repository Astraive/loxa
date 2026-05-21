class ResponseCapture:
    def __init__(self): self.status_code = 200; self.bytes = 0
    def record(self, status_code: int = 200, body_bytes: int = 0): self.status_code = status_code; self.bytes += body_bytes
