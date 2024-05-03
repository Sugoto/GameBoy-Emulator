#include <stdio.h>
#include <stdlib.h>
#include <winsock2.h>

#define PORT 12345
#define BUFFER_SIZE 256

int main() {
    WSADATA wsaData;
    SOCKET sock;
    struct sockaddr_in server;
    char buffer[BUFFER_SIZE];
    int recv_size;

    // Initialize Winsock
    if (WSAStartup(MAKEWORD(2, 2), &wsaData) != 0) {
        printf("WSAStartup failed.\n");
        return 1;
    }

    // Create a socket
    if ((sock = socket(AF_INET, SOCK_STREAM, 0)) == INVALID_SOCKET) {
        printf("Socket creation failed.\n");
        return 1;
    }

    // Specify server address and port
    server.sin_family = AF_INET;
    server.sin_addr.s_addr = inet_addr("127.0.0.1"); // Assuming the emulator is running on the same machine
    server.sin_port = htons(PORT);

    // Connect to server
    if (connect(sock, (struct sockaddr *)&server, sizeof(server)) < 0) {
        printf("Connection failed.\n");
        return 1;
    }

    printf("Connected to Gameboy emulator.\n");

    // Receive data from server
    while ((recv_size = recv(sock, buffer, BUFFER_SIZE, 0)) > 0) {
        printf("Received from Gameboy emulator: %s\n", buffer);
    }

    if (recv_size == SOCKET_ERROR) {
        printf("Receive failed.\n");
    }

    closesocket(sock);
    WSACleanup();

    return 0;
}
