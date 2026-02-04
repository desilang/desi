package com.desilang.plugin

import com.intellij.openapi.fileTypes.LanguageFileType
import com.intellij.openapi.util.IconLoader
import javax.swing.Icon

object DesiFileType : LanguageFileType(DesiLanguage.INSTANCE) {
    override fun getName() = "Desi"
    override fun getDescription() = "Desi language file"
    override fun getDefaultExtension() = "desi"
    override fun getIcon(): Icon = IconLoader.getIcon("/icons/desi.svg", DesiFileType::class.java)
}
