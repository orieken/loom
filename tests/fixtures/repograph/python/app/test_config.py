from app import config


def test_debug():
    assert not config.DEBUG
