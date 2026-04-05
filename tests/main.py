from http import client

import uuid, json, random, string, time, os
import threading, base64

import websocket

CLIENT_NUM = 2
MSG_NUM = 100

# intercept client send messages to catch and log everything
class Recorder:

    def __init__(self):
        # message sent by each client in order
        self.msgs = []

    def Record_msg(self, msg: dict):
        # print(msg)
        self.msgs.append({
            'sender': msg.get('Sender') or msg.get('UserID'),
            'content': msg.get('Content'),
            'timestamp': msg.get('Timestamp'),
        })

class Client(Recorder):
    url = 'ws://127.0.0.1:8080/ws/'

    def __init__(self):
        super().__init__()
        self.conn = client.HTTPConnection('127.0.0.1', 8080)

        self.username = uuid.uuid4()
        self.uid: str

        self.stop = threading.Event()

        # these two lists to track per client message send and recv
        self.msg_sent = []
        self.msg_recv = []

        self.ws = websocket.WebSocket()
        self.ws.settimeout(1)

        print(f'INFO: new CLIENT ID: {self.username}')

    def _register(self) -> str:
        self.conn.request('POST', '/api/register', json.dumps({
            'Username': str(self.username),
            'Password': 'mango'
        }))

        res = self.conn.getresponse()
        body = res.read()
        print(body, res.status)
        return json.loads(body).get('id')

    def _join_guild(self, guild_id: str):
        self.conn.request('POST', f'/api/{guild_id}', None, {
            'x-uid': self.uid,
        })
        self.conn.getresponse().read()

    def _login_guild(self, guild_id: str):
        self.conn.request('GET', f'/ws/{guild_id}/temp', None, {
            'x-uid': self.uid,
        })
        self.conn.getresponse().read()

    def _send_msgs(self, guild_id: str, channel_id: str):
        for i in range(0, MSG_NUM):
            msg = ''.join(random.choices(string.ascii_letters + string.digits, k=10))

            self.conn.request('POST', f'/api/{guild_id}/{channel_id}', json.dumps({
                'username': str(self.username),
                'content': msg,
            }), {
                'x-uid': self.uid,
                'content-type': 'application/json',
            })

            res = self.conn.getresponse()
            if res.status == 200:
                # print(f'{i}: Message {msg} sent by client {self.uid}')
                body = json.loads(res.read())
                # self.Record_msg(body)
                self.msg_sent.append(body)
            else:
                print(f'ERROR sending message: {res.status}')
                print(res.getheader('Location'))
                os._exit(1)
        return

    def _recv_msgs(self, signal: threading.Event):
        while not signal.is_set():
            try:
                msg = self.ws.recv()
                self.Record_msg(json.loads(msg))
                self.msg_recv.append(json.loads(msg))
            except websocket.WebSocketTimeoutException as e:
                continue
            except websocket.WebSocketConnectionClosedException as e:
                print('WS connection closed')
                break
            except Exception as e:
                print('ERROR:', e)
                break

    # first, initialize the client --> it needs to register and join guild
    # then, start sending and receiving messages (two threads)
    # send_msgs thread will return and thus die itself
    # recv_msgs needs to be killed through a signal
    def perform(self, guild_id: str) -> list[threading.Thread]:
        self.uid = self._register()
        self.ws.connect(self.url, header=[f'x-uid: {self.uid}'])
        self._join_guild(guild_id=guild_id)
        self._login_guild(guild_id=guild_id)

        e = threading.Event()
        t1 = threading.Thread(target=self._send_msgs, kwargs={
            'guild_id': guild_id,
            'channel_id': 'temp',
        })
        t2 = threading.Thread(target=self._recv_msgs, kwargs={
            'signal': e,
        })

        # t2.start()
        # t1.start()

        return [t1, t2]

        # t1.join() # wait for sending message to finish
        # time.sleep(2) # wait to ensure all messages are received
        # e.set()
        # t2.join()

def main():
    start_time = time.perf_counter()
    guild_id = '74759490-235e-48b2-8464-da1efc825ac0'

    threads: list[list[threading.Thread]] = []
    # recv_threads: list[threading.Thread] = []

    clients: list[Client] = []
    for _ in range(0, CLIENT_NUM):
        c = Client()
        clients.append(c)
        t1, t2 = c.perform(guild_id=guild_id)
        t2.start()
        threads.append([t1, t2])

    print('All clients have been initialized and have established \
WS connection.')
    print('ALL clients are listening for new messages as well.')
    input('Press any ENTER to start sending messages: ')

    for t in threads:
        t1, _ = t
        t1.start()

    print('ALL clients have sent all messages. Joining all sending \
threads.')

    for t in threads:
        t1, _ = t
        t1.join()

    print('ALL sending threads are joined. Waiting 5 seconds before \
closing WS connections to ensure all messages are received.')

    time.sleep(5)

    for c in clients:
        c.ws.close()

    print('Joining all receiving threads.')

    for t in threads:
        _, t2 = t
        t2.join()

    print('ALL receiving threads are joined.')

    base64_dumps = set()

    with open('tests/dump', 'w') as dump:
        for c in clients:
            print(f'Client: {c.uid}, messages received count: {len(c.msgs)}')
            jsondump = json.dumps(c.msgs, indent=2)
            dump.write(jsondump)
            hashed = base64.b64encode(jsondump.encode()).decode()
            base64_dumps.add(hashed)

    if len(base64_dumps) == 1:
        print('TEST PASSED: All clients received the same messages \
in the same order.')
    else:
        print('TEST FAILED: Clients did not receive the same messages \
in the same order.')
        
    for i in range(0, MSG_NUM):
        msgs = set()
        state = {}
        for c in clients:
            msgs.add(tuple(c.msgs[i]))
            state[c.uid] = c.msgs[i]
        if len(msgs) != 1:
            print('FAILED MESSAGE: ', msgs)
            print('STATE: ', state)
            break

    end_time = time.perf_counter()
    print(f'Time taken: {end_time - start_time:.4} seconds')

if __name__ == '__main__':
    main()
