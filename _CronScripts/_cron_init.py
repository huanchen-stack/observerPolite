from pymongo import MongoClient

series_name = "CronTenMilScan"
print(series_name, end=' ')

def get_all_collections():

    client = MongoClient("mongodb://localhost:27017")
    database = client["wikiPolite"]
    return database.list_collection_names()

collections = get_all_collections()
series = []
for collection_name in collections:
    if series_name in collection_name:
        series.append(collection_name)

series.sort()

print(series[-1])
