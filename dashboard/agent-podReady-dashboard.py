from elasticsearch import Elasticsearch
from touchstone import __version__
from touchstone.benchmarks.generic import Benchmark
from touchstone import decision_maker
from touchstone import databases
from touchstone.utils.lib import mergedicts, flatten_and_discard
import pandas as pd
from dash import Dash, dcc, html
import plotly.express as px
import os

database="elasticsearch"
es_url=os.getenv('es_url')
conn=databases.grab(database,es_url)

"""
Grab all the UUIDs from the last day
"""
uuid_query = { "query": { "range" : { "timestamp" : { "gte" : "now-3d"  } } }}

es = Elasticsearch(es_url)
benchmark=Benchmark(open("agent-podReady-config.json"), database)
conn=databases.grab(database,es_url)

import pprint

d=es.search(index="scaffolding",body=uuid_query,size=5000)
uuids = {}
sizes = {}
for id in d['hits']['hits']:
   print(id['_source']['action'])
   print(id['_source']['uuid'])
   if "agent" in id['_source']['action'] :
      uuid = id['_source']['uuid']
      uuids[uuid] = "{}_{}".format(id['_source']['cilium_version'],id['_source']['kernel'])
      sizes[uuid] = "{}".format(id['_source']['kernel'])

row_lists=[]
cpu_list=[]
for uuid in uuids :
    for compute in benchmark.compute_map['ripsaw-kube-burner'] :
        result=conn.emit_compute_dict(uuid,
                                      compute,
                                      "ripsaw-kube-burner",
                                      "uuid", uuid)
        flatten_and_discard(result,compute,row_lists)

latency=pd.DataFrame(data=row_lists,columns=["","metricName",
                                    "","uuid",
                                    "","name",
                                    "","result"])

pprint.pprint(latency)
pprint.pprint(row_lists)

def name_run(val,meta):
   return "{}_{}".format(meta[val],val[-4:])

def size_run(val,meta):
   return "{}_{}".format(meta[val],val[-4:])

latency['run'] = latency['uuid'].apply(lambda row: name_run(row,uuids))
latency['size'] = latency['uuid'].apply(lambda row: size_run(row,uuids))

app = Dash(__name__)
df = pd.pivot_table(latency,values='result',
               index='name',
               columns='run').to_dict()
fig = px.bar(df,
            y=list(df.keys()),
            x=['Ready','Initialized','PodScheduled','ContainersReady'],
            barmode='group',
            title="Pod Latency",
            )

app.layout = html.Div(children=[
   html.H1(children='Agent'),
   dcc.Graph(
      id='Latency',
      figure=fig
   )
])

app.run_server(host="0.0.0.0",debug=True)
