import fileinput
import zxcvbn

for line in fileinput.input():
    # test example descriptions
    if " " in line:
        continue

    # import paths
    if line.count("/") > 2 or line.startswith("github.com") or line.startswith("gopkg.in"):
        continue

    strength = zxcvbn.password_strength(line)
    entropy = strength.get("entropy")

    entropy_per_char = entropy/len(line)

    if entropy_per_char > 3.7:
        print("%f (%f) - %s" % (entropy, entropy_per_char, line.strip("\n")))
