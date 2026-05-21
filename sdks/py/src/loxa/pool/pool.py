from queue import SimpleQueue

class ObjectPool:
    def __init__(self, factory):
        self.factory = factory
        self.items = SimpleQueue()
    def get(self):
        return self.items.get() if not self.items.empty() else self.factory()
    def put(self, item):
        self.items.put(item)
