import fileinput
import re
import zxcvbn

GUID_REGEX = "[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-?[0-9A-F]{4}-[0-9A-F]{12}"

for line in fileinput.input():
    # test example descriptions
    if " " in line:
        continue

    # paths
    if line.count("/") > 2:
        continue

    # import paths
    if line.startswith("github.com") or line.startswith("gopkg.in"):
        continue

    # guid
    if re.match(GUID_REGEX, line, re.IGNORECASE):
        continue

    strength = zxcvbn.password_strength(line)
    entropy = strength.get("entropy")

    entropy_per_char = entropy/len(line)

    if entropy_per_char > 3.7:
        print("%f (%f) - %s" % (entropy, entropy_per_char, line.strip("\n")))
