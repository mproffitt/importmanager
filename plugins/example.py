import sys
import json
import os.path as path

arguments = json.loads(sys.argv[1])
print()
print(arguments["source"])
print(arguments["destination"])
print(arguments["details"])
print(arguments["properties"])

# Enable post processing
print(path.join(arguments["destination"], path.basename(arguments["source"])))
