from ..generated.spec_contract import CANONICAL_FIELDS


def is_canonical(key: str) -> bool:
    return key in CANONICAL_FIELDS
