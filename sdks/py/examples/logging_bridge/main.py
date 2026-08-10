import logging
from loza.integrations.logging import LozaHandler
logging.getLogger().addHandler(LozaHandler())
