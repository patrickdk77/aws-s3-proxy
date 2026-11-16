def pytest_addoption(parser):
    parser.addoption("--image", action="store", required=True)
