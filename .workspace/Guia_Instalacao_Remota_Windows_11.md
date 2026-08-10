# Guia Avançado: Instalação Remota e Automatizada do Windows 11 (Sem Pendrive)

Este documento descreve o procedimento técnico passo a passo para realizar uma instalação limpa do Windows 11 em uma máquina remota a partir de uma instalação existente do Windows, utilizando exclusivamente o arquivo ISO oficial e ferramentas nativas do sistema. 

O método baseia-se na criação de uma partição temporária de boot de onde o instalador será executado e na utilização de um arquivo de resposta (`autounattend.xml`) para automatizar completamente o processo de configuração e reativação do acesso remoto (RDP), eliminando a necessidade de intervenção humana local.

---

## ⚠️ Pré-requisitos e Avisos Importantes

* **Acesso Administrativo:** Você deve estar conectado à máquina remota com privilégios de Administrador.
* **Conexão de Rede Cabeada (Ethernet):** É fortemente recomendado que a máquina remota esteja conectada via cabo. Adaptadores Wi-Fi frequentemente exigem configurações adicionais ou drivers específicos que podem não ser carregados automaticamente durante a primeira inicialização do novo sistema, o que causaria a perda definitiva do acesso remoto.
* **Endereço IP Conhecido:** Certifique-se de que a máquina remota possui um IP estático (fixo) na rede local ou que você tenha acesso ao painel do roteador/servidor DHCP para identificar o novo IP que será atribuído à máquina pós-formatação.
* **Backup de Dados:** Este procedimento envolve a formatação da partição principal (`C:`). Todos os dados contidos nela serão apagados. Certifique-se de salvar arquivos importantes em outra unidade ou na nuvem antes de prosseguir.
* **Compatibilidade de Hardware:** Se a máquina remota não atender aos requisitos oficiais do Windows 11 (TPM 2.0, Secure Boot, CPU compatível), os passos adicionais para ignorar essa verificação devem ser injetados no arquivo de resposta ou aplicados previamente.

---

## Passo 1: Criação do Arquivo de Resposta Automatizado (`autounattend.xml`)

Como a conexão remota via softwares comerciais (AnyDesk, TeamViewer) será cortada assim que o computador for reiniciado, o instalador do Windows precisa trabalhar sozinho. O arquivo `autounattend.xml` instruirá o instalador a pular todas as perguntas iniciais, formatar o disco, criar o usuário e ativar o protocolo RDP.

1.  Na sua máquina local (ou na própria máquina remota, caso tenha acesso ao navegador), acesse o site gerador de arquivos de resposta (por exemplo, [Windows Answer File Generator](https://www.windowsafg.com/win11x64_uefi.html)).
2.  Configure os seguintes parâmetros essenciais:
    * **Language (Idioma):** Selecione o idioma correspondente à ISO do Windows 11 baixada (ex: `pt-BR`).
    * **Keyboard (Teclado):** Selecione o layout correto (ex: `Portuguese (Brazilian ABNT2)`).
    * **Product Key (Chave de Produto):** Insira sua chave ou utilize uma chave genérica de instalação da Microsoft apenas para avançar o instalador (ex: Chave genérica do Windows 11 Pro: `VK7JG-NPHTM-C97JM-9MPGT-3V66T`).
    * **Computer Name (Nome do Computador):** Defina um nome de sua escolha.
    * **User Accounts (Contas de Usuário):** Crie uma conta de Administrador (ex: Usuário: `SuporteRemoto` / Senha: `UmaSenhaForteSegura123`). **Anote estas credenciais.**
    * **Partition Settings (Configurações de Partição):** Defina para instalar na partição existente do Windows (`C:`). No gerador de XML, certifique-se de configurar para formatar apenas a partição onde o sistema operacional antigo reside, preservando a nova partição temporária que criaremos no Passo 3.
    * **Remote Desktop (Área de Trabalho Remota):** **[OBRIGATÓRIO]** Marque a opção para **Habilitar o Remote Desktop (RDP)** e permitir conexões através do Firewall do Windows.
3.  Gere e baixe o arquivo. Certifique-se de salvá-lo com o nome exato: `autounattend.xml`.

---

## Passo 2: Montagem da Imagem ISO no Sistema Atual

O Windows consegue mapear e ler arquivos ISO nativamente sem softwares de terceiros. Faremos isso através do PowerShell para garantir precisão no ambiente remoto.

1.  Abra o **PowerShell como Administrador** na máquina remota.
2.  Execute o comando abaixo, substituindo o caminho pelo local real onde a ISO do Windows 11 está armazenada:
    ```powershell
    Mount-DiskImage -ImagePath "C:\Caminho\Ate\O\Arquivo\Windows11.iso"
    ```
3.  Para descobrir qual letra de unidade virtual foi atribuída à ISO montada, execute o comando:
    ```powershell
    Get-Volume
    ```
4.  Identifique na lista a unidade cujo tipo seja `CD-ROM` ou que possua o tamanho aproximado da imagem do Windows (entre 5 GB e 6 GB). Anote esta letra (neste guia, assumiremos que a ISO assumiu a letra **`E:`**).

---

## Passo 3: Criação da Partição de Instalação Temporária (`T:`)

Reduziremos o volume principal (`C:`) em aproximadamente 10 GB para criar uma partição dedicada e isolada de onde os arquivos do instalador serão lidos após o reboot.

1.  Ainda na máquina remota, abra o **Prompt de Comando (CMD) como Administrador**.
2.  Inicie o utilitário de particionamento nativo:
    ```cmd
    diskpart
    ```
3.  Liste os discos disponíveis para identificar o disco principal (geralmente é o Disco 0):
    ```cmd
    list disk
    ```
4.  Selecione o disco principal:
    ```cmd
    select disk 0
    ```
5.  Liste as partições para identificar qual é a partição principal do sistema operacional atual (geralmente rotulada como "Principal" ou com o maior tamanho em GB):
    ```cmd
    list partition
    ```
6.  Selecione a partição principal encontrada (substitua o número `X` pelo número correspondente à sua partição `C:`):
    ```cmd
    select partition X
    ```
7.  Diminua o volume atual em 10.000 MB (aproximadamente 10 GB):
    ```cmd
    shrink desired=10000
    ```
8.  Crie uma nova partição primária ocupando o espaço recém-liberado:
    ```cmd
    create partition primary
    ```
9.  Formate rapidamente a nova partição no sistema de arquivos NTFS e atribua um rótulo identificador:
    ```cmd
    format fs=ntfs quick label="InstaladorWin11"
    ```
10. Atribua a letra **`T`** para esta nova partição:
    ```cmd
    assign letter=T
    ```
11. Saia do utilitário Diskpart:
    ```cmd
    exit
    ```

---

## Passo 4: Transferência dos Arquivos de Instalação e Injeção do XML

Com a nova partição estruturada, moveremos os arquivos internos da ISO e aplicaremos a automação.

1.  No Prompt de Comando (Administrador), execute o comando abaixo para copiar integralmente todos os arquivos e pastas da ISO montada (`E:`) para a nova partição temporária (`T:`):
    ```cmd
    xcopy E:\*.* T:\ /E /H /F
    ```
2.  Pegue o arquivo `autounattend.xml` criado no **Passo 1** e transfira-o para a máquina remota.
3.  Mova ou copie esse arquivo para dentro da pasta `sources` localizada na partição `T:`. O caminho final absoluto deve ser exatamente:
    ```text
    T:\sources\autounattend.xml
    ```
    *Nota: É crucial que o arquivo esteja dentro da pasta `sources` da unidade de instalação, pois o ambiente de pré-instalação do Windows (WinPE) busca automaticamente por este arquivo nesta árvore de diretórios.*

---

## Passo 5: Configuração do Menu de Inicialização (BCD) para o Boot Remoto

Configuraremos o gerenciador de boot do Windows para carregar o ambiente de instalação diretamente da partição `T:` no próximo reinício do sistema, ignorando o carregamento do Windows atual.

1.  No Prompt de Comando (Administrador), crie uma cópia da entrada de boot atual do sistema para servir de base para o nosso instalador:
    ```cmd
    bcdedit /copy {current} /d "Instalacao Remota Windows 11"
    ```
2.  Ao executar o comando acima, o sistema exibirá uma mensagem de confirmação contendo um identificador exclusivo de 128 bits chamado GUID, delimitado por chaves. Exemplo:
    ```text
    A entrada foi copiada com êxito para {cbd971bf-1234-1234-1234-123456789abc}.
    ```
    **Copie exatamente o GUID gerado na sua tela (incluindo as chaves `{ }`).**

3.  Configure os parâmetros dessa nova entrada de boot para apontar para os arquivos de instalação na partição `T:`. Execute os três comandos abaixo substituindo `{SEU_GUID_AQUI}` pelo código copiado no subpasso anterior:
    ```cmd
    bcdedit /set {SEU_GUID_AQUI} device partition=T:
    bcdedit /set {SEU_GUID_AQUI} osdevice partition=T:
    bcdedit /set {SEU_GUID_AQUI} path \sources\boot.wim
    ```
4.  Defina o sistema para inicializar obrigatoriamente e de forma direta nesta nova opção criada na próxima vez que o computador ligar:
    ```cmd
    bcdedit /bootsequence {SEU_GUID_AQUI}
    ```

---

## Passo 6: Execução do Reboot e Acompanhamento pós-Instalação

Com toda a estrutura de disco, automação e inicialização devidamente amarrada, o processo está pronto para ser disparado.

1.  Certifique-se de que nenhum outro comando ou arquivo esteja pendente de salvamento na máquina remota.
2.  No Prompt de Comando, execute a instrução de reinicialização imediata e forçada:
    ```cmd
    shutdown /r /f /t 0
    ```

### O que ocorrerá nos bastidores (Fase Cega)
* Sua sessão atual do AnyDesk / TeamViewer / RDP será encerrada imediatamente.
* A máquina remota reiniciará e, devido ao comando `/bootsequence`, entrará direto no ambiente contido em `T:\sourcesoot.wim`.
* O assistente de instalação detectará o arquivo `T:\sourcesutounattend.xml`.
* A automação apagará a antiga partição `C:`, instalará o Windows 11, aplicará as configurações regionais, criará a conta de administrador definida e liberará o serviço RDP no Firewall.
* Esse procedimento costuma demorar entre 15 e 35 minutos, variando de acordo com as especificações de hardware (processador e velocidade de leitura/escrita do SSD).

---

## Passo 7: Retomando o Controle Remoto

1.  Na sua máquina local, abra o utilitário nativo **Conexão de Área de Trabalho Remota** (pressione `Win + R`, digite `mstsc` e pressione Enter).
2.  No campo "Computador", digite o **Endereço IP** da máquina remota.
3.  Clique em **Conectar**.
4.  Quando o sistema solicitar as credenciais, insira o **Usuário** e a **Senha** configurados no arquivo `autounattend.xml` durante o Passo 1.
5.  Ao logar com sucesso, você estará na Área de Trabalho do novo Windows 11. 
6.  *(Opcional)* Agora você pode abrir novamente o Gerenciamento de Disco (`diskmgmt.msc`), excluir a partição temporária `T:` e estender a partição `C:` para recuperar os 10 GB utilizados no processo.
