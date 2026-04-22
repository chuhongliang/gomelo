/**
 * Gomelo Cocos Client Demo
 * Place this script on a Node in your Cocos Creator 3.x scene
 */

import { _decorator, Component } from 'cc';
import { GomeloClient } from '../GomeloClient';

const { ccclass, property } = _decorator;

@ccclass('Demo')
export class Demo extends Component {

    private client: GomeloClient | null = null;

    start() {
        this.client = new GomeloClient();
        this.client.host = 'localhost';
        this.client.port = 3010;
        this.client.timeout = 5000;
        this.client.heartbeatInterval = 30000;

        this.client.onConnected = () => console.log('Connected');
        this.client.onDisconnected = () => console.log('Disconnected');
        this.client.onError = (err) => console.error('Error:', err);

        this.client.on('onChat', (data) => console.log('Chat:', data));
        this.client.on('onPlayerJoin', (data) => console.log('PlayerJoin:', data));

        this.client.connect();

        this.scheduleOnce(() => {
            this.makeRequest();
        }, 1);
    }

    makeRequest() {
        if (!this.client) return;

        this.client.registerRoute('connector.entry', 1);
        this.client.registerRoute('player.move', 2);

        this.client.request('connector.entry', { name: 'Player1' })
            .then((data) => console.log('Response:', data))
            .catch((err) => console.error('Request failed:', err));

        this.client.notify('player.move', { x: 100, y: 200 });
    }

    onDestroy() {
        this.client?.disconnect();
    }
}