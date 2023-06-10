import sys
import json

arguments = json.loads(sys.argv[1])
print()
print(arguments["source"])
print(arguments["destination"])
print(arguments["details"])
print(arguments["properties"])
