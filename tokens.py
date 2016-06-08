import sys

from pygments import lexers
from pygments.token import Token


def get_tokens(filename):
    with open(sys.argv[1]) as f:
        content = f.read()

        try:
            lexer = lexers.get_lexer_for_filename(sys.argv[1])
        except:
            lexer = lexers.guess_lexer(content)

        return lexer.get_tokens(content)


def interesting_token(typee, content):
    return typee in Token.String and content != '"' and content != "'"


def password_token(content):
    return len(content) >= 10 and len(content) < 48


def strip(content):
    if content[0] == "'" and content[-1] == "'":
        return content[1:-1]

    if content[0] == '"' and content[-1] == '"':
        return content[1:-1]

    return content

tokens = get_tokens(sys.argv[1])

candidates = [strip(content) for (typee, content) in tokens
              if interesting_token(typee, content) and password_token(content)]

for candidate in candidates:
    print(candidate)
