import logging
from loxa.integrations.logging import LoxaHandler
logging.getLogger().addHandler(LoxaHandler())
