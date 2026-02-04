package com.desilang.plugin

import com.redhat.devtools.lsp4ij.server.ProcessStreamConnectionProvider
import com.redhat.devtools.lsp4ij.server.StreamConnectionProvider
import com.redhat.devtools.lsp4ij.LanguageServerFactory
import com.intellij.openapi.project.Project

class DesiLspServerFactory : LanguageServerFactory {
    override fun createConnectionProvider(project: Project): StreamConnectionProvider {
        // Try to find desilsp in PATH or use configured path
        val command = listOf("desilsp")
        return ProcessStreamConnectionProvider(command)
    }
}
