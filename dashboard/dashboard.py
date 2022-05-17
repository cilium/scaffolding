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
uuid_query = { "query": { "range" : { "uperf_ts" : { "gte" : "now-3d", "lt" :  "now" } } }
     ,"aggs": { "name": { "terms": { "field": "uuid.keyword" } } } }

es = Elasticsearch(es_url)
benchmark=Benchmark(open("config.json"), database)
conn=databases.grab(database,es_url)

d=es.search(index="ripsaw-uperf-results",body=uuid_query)
uuids = [value['key'] for value in d['aggregations']['name']['buckets']]

row_lists=[]
cpu_list=[]
for uuid in uuids :
    for compute in benchmark.compute_map['system-metrics'] :
        smetrics=conn.emit_compute_dict(uuid,compute,'system-metrics','uuid',uuid)
        flatten_and_discard(smetrics,compute,cpu_list)
    for compute in benchmark.compute_map['ripsaw-uperf-results'] :
        result=conn.emit_compute_dict(uuid,
                                      compute,
                                      "ripsaw-uperf-results",
                                      "uuid", uuid)
        flatten_and_discard(result,compute,row_lists)

data=pd.DataFrame(data=row_lists,columns=["","test_type",
                                          "","run_id",
                                          "","uuid",
                                          "","protocol",
                                          "","message_size",
                                          "","num_threads",
                                          "","num_pairs",
                                          "data","result"])

cpudata=pd.DataFrame(data=cpu_list,columns=["","metric",
                                          "","uuid",
                                          "","node",
                                          "","mode",
                                          "","value"])


stream=data.loc[(data['test_type']=="stream") &
       (data['protocol']=="tcp") &
       (data['data']=="avg(norm_byte)")&
       (data['num_threads']==1)]
stream_udp=data.loc[(data['test_type']=="stream") &
       (data['protocol']=="udp") &
       (data['data']=="avg(norm_byte)")&
       (data['num_threads']==1)]
rr=data.loc[(data['test_type']=="rr") &
       (data['protocol']=="tcp") &
       (data['data']=="avg(norm_ltcy)")&
       (data['num_threads']==1)]
rr_udp=data.loc[(data['test_type']=="rr") &
       (data['protocol']=="udp") &
       (data['data']=="avg(norm_ltcy)")&
       (data['num_threads']==1)]

df = pd.pivot_table(stream,values='result',
               index='message_size',
               columns='uuid').to_dict()
app = Dash(__name__)
fig = px.bar(df,
            x=list(df.keys()),
            y=['64','1024','16384'],
            barmode='group',
            title="TCP Stream",
            )
df = pd.pivot_table(stream_udp,values='result',
               index='message_size',
               columns='uuid').to_dict()

udpfig = px.bar(df,
            x=list(df.keys()),
            y=['64','1024','16384'],
            barmode='group',
            title="UDP Stream",
            )

df = pd.pivot_table(rr,values='result',
               index='message_size',
               columns='uuid').to_dict()

rrfig = px.bar(df,
            x=list(df.keys()),
            y=['64','1024','16384'],
            barmode='group',
            title="TCP RR"
            )

df = pd.pivot_table(cpudata,values='value',
               index='mode',
               columns='uuid').to_dict()

cpu_usage = px.bar(df,
                  barmode='group',
                  title="CPU Usage")

app.layout = html.Div(children=[
   html.H1(children='Datapath Performance'),
   dcc.Graph(
      id='TCP Stream',
      figure=fig
   ),
      dcc.Graph(
      id='UDP Stream',
      figure=udpfig
   ),
      dcc.Graph(
      id='TCP RR',
      figure=rrfig
   ),
      dcc.Graph(
         id='CPU Usage',
         figure=cpu_usage
      ),
])

app.run_server(host="0.0.0.0",debug=True)
