"""Pygments lexer for the Desi programming language."""

from pygments.lexer import RegexLexer, bygroups, words
from pygments.token import (
    Comment, Keyword, Name, String, Number, Operator, Punctuation, Text
)

class DesiLexer(RegexLexer):
    """Pygments lexer for Desi source code."""
    
    name = 'Desi'
    aliases = ['desi']
    filenames = ['*.desi']
    mimetypes = ['text/x-desi']

    # Keywords
    KEYWORDS = (
        'def', 'let', 'mut', 'if', 'elif', 'else', 'for', 'in', 'while',
        'match', 'return', 'break', 'continue', 'pass', 'import', 'from',
        'as', 'class', 'struct', 'enum', 'trait', 'impl', 'pub', 'self',
        'and', 'or', 'not', 'is', 'True', 'False', 'None', 'async', 'await',
        'using', 'defer', 'unsafe', 'spawn', 'select', 'lambda', 'raise',
        'try', 'except', 'finally', 'with', 'type', 'where'
    )

    # Built-in types
    BUILTINS = (
        'int', 'i8', 'i16', 'i32', 'i64', 'u8', 'u16', 'u32', 'u64',
        'float', 'f32', 'f64', 'bool', 'str', 'list', 'dict', 'set',
        'tuple', 'Option', 'Result', 'Some', 'None', 'Ok', 'Err',
        'cptr', 'ref', 'inout'
    )

    # Decorators
    DECORATORS = (
        'extern', 'ffi_struct', 'inline', 'deprecated', 'test'
    )

    tokens = {
        'root': [
            # Comments
            (r'#.*$', Comment.Single),
            
            # Strings
            (r'f"', String, 'fstring'),
            (r'"""', String, 'docstring'),
            (r'"', String, 'string'),
            
            # Numbers
            (r'0x[0-9a-fA-F_]+', Number.Hex),
            (r'0b[01_]+', Number.Bin),
            (r'0o[0-7_]+', Number.Oct),
            (r'\d+\.\d*([eE][+-]?\d+)?', Number.Float),
            (r'\d+[eE][+-]?\d+', Number.Float),
            (r'\d[\d_]*', Number.Integer),
            
            # Decorators
            (r'@(\w+)', bygroups(Name.Decorator)),
            
            # Keywords
            (words(KEYWORDS, suffix=r'\b'), Keyword),

            # Built-in types
            (words(BUILTINS, suffix=r'\b'), Keyword.Type),
            
            # Function definitions
            (r'(def)(\s+)(\w+)', bygroups(Keyword, Text, Name.Function)),
            (r'(class|struct|enum|trait)(\s+)(\w+)', bygroups(Keyword, Text, Name.Class)),
            
            # Operators
            (r'->|=>|:=|\|>|::|\.\.\.?', Operator),
            (r'[+\-*/%=<>!&|^~]+', Operator),
            
            # Punctuation
            (r'[\[\](){}:,.]', Punctuation),
            
            # Identifiers
            (r'\b[A-Z]\w*\b', Name.Class),  # Type names start with uppercase
            (r'\b\w+\b', Name),
            
            # Whitespace
            (r'\s+', Text),
        ],
        'string': [
            (r'\\[nrt\\"\']', String.Escape),
            (r'[^"\\]+', String),
            (r'"', String, '#pop'),
        ],
        'fstring': [
            (r'\{[^}]+\}', String.Interpol),
            (r'\\[nrt\\"\']', String.Escape),
            (r'[^"\\{]+', String),
            (r'"', String, '#pop'),
        ],
        'docstring': [
            (r'"""', String, '#pop'),
            (r'[^"]+', String),
            (r'"', String),
        ],
    }
