from setuptools import setup, find_packages

setup(
    name="pysysml",
    version="0.1.0",
    description="Python client library for Systemica SysML v2 parser",
    author="Open-MBEE",
    packages=find_packages(),
    python_requires=">=3.8",
    install_requires=[
        "grpcio>=1.83.0",
        "protobuf>=7.35.1",
    ],
    extras_require={
        "dev": [
            "pytest>=7.0.0",
            "pytest-mock>=3.10.0",
            "grpcio-tools>=1.83.0",
        ],
    },
)
