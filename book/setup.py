"""Setup for Desi Pygments lexer."""

from setuptools import setup

setup(
    name='pygments-desi',
    version='0.1.0',
    description='Pygments lexer for the Desi programming language',
    packages=['pygments_desi'],
    entry_points={
        'pygments.lexers': [
            'desi = pygments_desi:DesiLexer',
        ],
    },
    install_requires=['pygments'],
)
