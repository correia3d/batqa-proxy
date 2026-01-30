# BATQA Proxy

Proxy TCP transparente para acelerar comandos ServerQuery do TeamSpeak/TeaSpeak.

## 🎯 O Problema

Quando o BATQA App envia comandos para um servidor distante, cada comando tem latência de rede:

```
App → Internet (100ms) → TeamSpeak → Internet (100ms) → App
                    Total: ~200ms por comando
```

Para 100 comandos = **20 segundos** de espera!

## 💡 A Solução

O BATQA Proxy roda **no mesmo servidor** que o TeamSpeak, eliminando a latência entre o proxy e o TS:

```
App → Internet (100ms) → Proxy → localhost (0ms) → TeamSpeak
```

### Batch de Comandos

O App envia múltiplos comandos de uma vez, o Proxy executa todos localmente:

```
┌─────────────────┐                    ┌─────────────────────────────────┐
│  BATQA App      │   1 pacote TCP     │  Servidor                       │
│                 │   (~100ms)         │                                 │
│  100 comandos   │ ─────────────────▶ │  Proxy ──(0ms)──▶ TeamSpeak     │
│  enviados junto │                    │    │                            │
│                 │ ◀───────────────── │    └── Executa 100 comandos     │
│  100 respostas  │   1 pacote TCP     │        instantaneamente!        │
│  recebidas      │   (~100ms)         │                                 │
└─────────────────┘                    └─────────────────────────────────┘

Total: ~200ms para 100 comandos (em vez de 20 segundos!)
```

## 📊 Ganho de Performance

| Operação | Sem Proxy | Com Proxy | Melhoria |
|----------|-----------|-----------|----------|
| 1 comando | 200ms | 200ms | - |
| 10 comandos | 2 seg | 200ms | **10x** |
| 50 comandos | 10 seg | 200ms | **50x** |
| 100 comandos | 20 seg | 200ms | **100x** |
| 1000 comandos | 3.3 min | 300ms | **660x** |

### Casos de Uso

| Ação | Comandos | Tempo Atual | Com Proxy |
|------|----------|-------------|-----------|
| Kick 50 usuários | 50 | ~10 seg | ~200ms |
| Poke todos (100) | 100 | ~20 seg | ~200ms |
| Mover canal (30) | 30 | ~6 seg | ~200ms |
| Mensagem privada (50) | 50 | ~10 seg | ~200ms |
| Backup permissões | 200+ | ~40 seg | ~300ms |

## 🔧 Instalação

### Requisitos

- Linux (64-bit)
- TeamSpeak 3 Server (porta 10011) ou TeaSpeak (porta 10101)
- Git instalado

### Instalação Automática (Recomendado)

O script de instalação faz tudo automaticamente:
- Compila o binário (se Go estiver instalado) ou baixa pré-compilado
- Detecta TeamSpeak ou TeaSpeak
- Permite configurar porta personalizada do ServerQuery
- Cria e inicia o serviço systemd

```bash
git clone https://github.com/correia3d/batqa-proxy.git
cd batqa-proxy
sudo ./install.sh
```

O instalador vai perguntar:
- **Porta do proxy**: onde o BATQA vai conectar (padrão: 10202 para TS, 10203 para TeaSpeak)
- **Porta do ServerQuery**: porta do seu servidor (padrão: 10011 para TS, 10101 para TeaSpeak)

> 💡 Se seu TeamSpeak usa uma porta personalizada, basta informar durante a instalação!

### Portas Padrão

| Servidor | Porta ServerQuery | Porta Proxy |
|----------|-------------------|-------------|
| TeamSpeak 3 | 10011 | 10202 |
| TeaSpeak | 10101 | 10203 |

### Execução Manual (Opcional)

Se preferir rodar manualmente sem systemd:

```bash
# TeamSpeak (padrão)
./batqa-proxy -listen :10202 -target localhost:10011

# TeaSpeak
./batqa-proxy -listen :10203 -target localhost:10101

# Porta personalizada do TeamSpeak (ex: 10022)
./batqa-proxy -listen :10202 -target localhost:10022
```

### Parâmetros

| Parâmetro | Padrão | Descrição |
|-----------|--------|-----------|
| `-listen` | `:10202` | Porta que o proxy escuta |
| `-target` | `localhost:10011` | Endereço do ServerQuery |
| `-max-conns` | `100` | Máximo de conexões simultâneas |
| `-timeout` | `30s` | Timeout de conexão |
| `-rate-limit` | `100` | Máximo de comandos por segundo por IP |
| `-log` | `info` | Nível de log (debug, info, warn, error) |

### Gerenciamento do Serviço

O `install.sh` cria o serviço automaticamente. Comandos úteis:

```bash
# Ver status
sudo systemctl status batqa-proxy

# Ver logs
sudo journalctl -u batqa-proxy -f

# Reiniciar
sudo systemctl restart batqa-proxy

# Parar
sudo systemctl stop batqa-proxy

# Desinstalar
sudo systemctl stop batqa-proxy
sudo systemctl disable batqa-proxy
sudo rm /etc/systemd/system/batqa-proxy.service
sudo rm /usr/local/bin/batqa-proxy
```

### Firewall

```bash
# Liberar porta no firewall
sudo ufw allow 10202/tcp

# Ou com iptables
sudo iptables -A INPUT -p tcp --dport 10202 -j ACCEPT
```

## 🔒 Segurança

O proxy oferece a **mesma segurança** que expor a porta 10011 diretamente:

- Autenticação é feita pelo TeamSpeak (login/senha)
- Senhas trafegam da mesma forma que na 10011
- Mesma superfície de ataque

### Medidas de Proteção Incluídas

1. **Rate Limiting**: Máximo de comandos por segundo por IP
2. **Timeout**: Conexões inativas são fechadas
3. **Max Connections**: Limite de conexões simultâneas
4. **Logging**: Registro de todas as conexões

### Recomendações

- Use senhas fortes no ServerQuery
- Considere usar VPN para conexões sensíveis
- Monitore os logs regularmente

## 📱 Configuração no BATQA App

No BATQA, ao adicionar/editar um servidor:

```
Host: seu-servidor.com
Porta: 10202          ← Porta do proxy (em vez de 10011)
Usuário: serveradmin
Senha: sua-senha
```

**Nenhuma outra mudança necessária!** O proxy é 100% transparente.

## 🔍 Como Funciona

### Fluxo de Dados

```
1. BATQA conecta no Proxy (porta 10012)
2. Proxy abre conexão com TS local (porta 10011)
3. Tudo que BATQA envia → Proxy repassa pro TS
4. Tudo que TS responde → Proxy repassa pro BATQA
5. Conexão encerra → Proxy fecha ambas as pontas
```

### Batch de Comandos

O protocolo ServerQuery usa `\n` como separador. O BATQA pode enviar:

```
clientkick clid=1 reasonid=5\n
clientkick clid=2 reasonid=5\n
clientkick clid=3 reasonid=5\n
```

O Proxy recebe tudo em um pacote TCP e executa cada linha instantaneamente no TS local.

### Pool de Conexões (Opcional)

O proxy pode manter conexões pré-abertas com o TS para eliminar até o tempo de handshake TCP local.

## 📈 Estatísticas (Futuro)

O proxy pode coletar métricas enquanto roda 24/7:

- Usuários online ao longo do tempo
- Pico de usuários por dia
- Histórico de canais
- Logs de conexão

Essas estatísticas ficarão disponíveis via API REST para o BATQA exibir gráficos.

## 🐛 Troubleshooting

### Proxy não conecta no TS

```bash
# Verificar se TS está rodando
netstat -tlnp | grep 10011

# Testar conexão manual
telnet localhost 10011
```

### Conexão recusada

```bash
# Verificar firewall
sudo ufw status
sudo iptables -L -n | grep 10012
```

### Verificar logs

```bash
# Se rodando como serviço
journalctl -u batqa-proxy -f

# Se rodando manual
./batqa-proxy -log debug
```

## 📝 Licença

MIT License - Use livremente!

## 🤝 Contribuição

Pull requests são bem-vindos! Para mudanças grandes, abra uma issue primeiro.

---

**BATQA Modern** - TeamSpeak Query Admin Tool
