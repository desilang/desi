const { LanguageClient, TransportKind } = require('vscode-languageclient/node');
const path = require('path');
const { workspace } = require('vscode');

let client;

function activate(context) {
    // Find desilsp binary - check common locations
    const config = workspace.getConfiguration('desi');
    let serverPath = config.get('lspPath');

    if (!serverPath) {
        // Try to find in PATH or common locations
        serverPath = 'desilsp';
    }

    const serverOptions = {
        run: { command: serverPath, transport: TransportKind.stdio },
        debug: { command: serverPath, args: ['--log', '/tmp/desilsp.log'], transport: TransportKind.stdio }
    };

    const clientOptions = {
        documentSelector: [{ scheme: 'file', language: 'desi' }],
        synchronize: {
            fileEvents: workspace.createFileSystemWatcher('**/*.desi')
        }
    };

    client = new LanguageClient(
        'desilsp',
        'Desi Language Server',
        serverOptions,
        clientOptions
    );

    client.start();
}

function deactivate() {
    if (client) {
        return client.stop();
    }
}

module.exports = { activate, deactivate };
