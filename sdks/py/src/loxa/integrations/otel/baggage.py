from ...core.attr import String

def Baggage(**items):
    return [String(f"baggage.{k}", str(v)) for k, v in items.items()]
