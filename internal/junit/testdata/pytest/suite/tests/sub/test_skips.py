import pytest


def test_passes():
    assert True


@pytest.mark.skip(reason="example")
def test_skipped():
    pass
