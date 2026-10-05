"""Entry point.

Docs may mention `import fake.thing` without importing it.
"""
import os
import requests
import app.services.billing
from . import config
from .models import user
from app.services import billing as b


def run():
    # import also_fake
    return os.getcwd()
