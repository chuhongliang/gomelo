using UnityEngine;
using Gomelo;
using Gomelo.Network;

public class Demo : MonoBehaviour
{
    private GomeloClient client;

    void Start()
    {
        client = gameObject.AddComponent<GomeloClient>();
        client.Host = "localhost";
        client.Port = 3010;
        client.Timeout = 5000;
        client.HeartbeatInterval = 30000;

        client.OnConnected += () => Debug.Log("Connected");
        client.OnDisconnected += () => Debug.Log("Disconnected");
        client.OnError += (err) => Debug.LogError("Error: " + err);
        client.OnResponse += (seq, data) => Debug.Log($"Response {seq}: {data}");
        client.OnNotify += (route, data) => Debug.Log($"Notify {route}: {data}");

        client.On("onChat", (data) => Debug.Log("Chat: " + data));
        client.On("onPlayerJoin", (data) => Debug.Log("PlayerJoin: " + data));

        client.RegisterRoute("connector.entry", 1);
        client.RegisterRoute("player.move", 2);

        client.Connect();

        StartCoroutine(RequestAfterConnected());
    }

    System.Collections.IEnumerator RequestAfterConnected()
    {
        yield return new WaitUntil(() => client.IsConnected);
        Debug.Log("Making request...");

        client.Request("connector.entry", new { name = "Player1" },
            (data) => Debug.Log("Entry success: " + data),
            (err) => Debug.LogError("Entry failed: " + err));

        client.Notify("player.move", new { x = 100, y = 200 });
    }

    void OnDestroy()
    {
        client?.Disconnect();
    }
}