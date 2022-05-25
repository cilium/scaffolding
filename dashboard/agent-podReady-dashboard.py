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
uuid_query = { "query": { "range" : { "timestamp" : { "gte" : "now-3d", "lt" :  "now" } } }
     ,"aggs": { "name": { "terms": { "field": "uuid.keyword" } } } }

es = Elasticsearch(es_url)
benchmark=Benchmark(open("agent-podReady-config.json"), database)
conn=databases.grab(database,es_url)

d=es.search(index="ripsaw-kube-burner",body=uuid_query)
uuids = [value['key'] for value in d['aggregations']['name']['buckets']]

row_lists=[]
cpu_list=[]
for uuid in uuids :
    for compute in benchmark.compute_map['ripsaw-kube-burner'] :
        result=conn.emit_compute_dict(uuid,
                                      compute,
                                      "ripsaw-kube-burner",
                                      "uuid", uuid)
        flatten_and_discard(result,compute,row_lists)

import pprint

pprint.pprint(row_lists)
latency=pd.DataFrame(data=row_lists,columns=["","metricName",
                                    "","uuid",
                                    "","name",
                                    "","result"])

pprint.pprint(latency)

app = Dash(__name__)

df = pd.pivot_table(latency,values='result',
               index='metricName',
               columns='uuid').to_dict()
fig = px.bar(df,
            x=list(df.keys()),
            y=['Ready'],
            barmode='group',
            title="",
            )

app.layout = html.Div(children=[
   html.H1(children='Agent'),
   dcc.Graph(
      id='Latency',
      figure=fig
   )
])

app.run_server(host="0.0.0.0",debug=True)
