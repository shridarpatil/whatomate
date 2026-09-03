package database

import (
	"fmt"
	"strings"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/zerodha/logf"
	"gorm.io/gorm"
)

// grantRule liga uma capacidade que o papel já tem à permissão que ele ganha.
type grantRule struct {
	fromResource string
	fromAction   string
	toResource   string
	toAction     string
}

// occurrenceGrants é a equivalência exata com a capacidade de hoje. Quem
// administra etapas neste momento é quem tem settings.general:write, e é só
// esse que recebe as permissões de funil — roles:write não entra, porque
// concedê-lo alargaria acesso numa migração que promete não alterar o de
// ninguém.
var occurrenceGrants = []grantRule{
	{models.ResourceChat, models.ActionRead, models.ResourceOccurrences, models.ActionRead},
	{models.ResourceChat, models.ActionWrite, models.ResourceOccurrences, models.ActionWrite},
	// As três de etapa, incluindo read: sem ela a guarda de rota do frontend
	// esconde a tela de configuração até de quem pode editar.
	//
	// NÃO CORRIGIR sem decidir antes: um papel com settings.general:write e
	// SEM nenhuma permissão de chat recebe estas três mas não
	// occurrences:read, então ele abre a tela de configuração de etapas e a
	// chamada que lista etapas devolve 403. É proposital — o backfill
	// concede por equivalência com a capacidade de hoje, e esse papel não
	// tinha nenhum acesso ao CRM no dia anterior; conceder-lhe
	// occurrences:read alargaria acesso numa migração que promete não
	// alterar o de ninguém.
	{models.ResourceSettingsGeneral, models.ActionWrite, models.ResourceOccurrenceStages, models.ActionRead},
	{models.ResourceSettingsGeneral, models.ActionWrite, models.ResourceOccurrenceStages, models.ActionWrite},
	{models.ResourceSettingsGeneral, models.ActionWrite, models.ResourceOccurrenceStages, models.ActionDelete},
}

// occurrencePermissionKeys são as permissões que este backfill distribui. A
// guarda de "já foi semeado" compara contra esta lista em vez do tamanho de
// occurrenceGrants, que tem entradas repetidas por origem.
var occurrencePermissionKeys = []string{
	models.ResourceOccurrences + ":" + models.ActionRead,
	models.ResourceOccurrences + ":" + models.ActionWrite,
	models.ResourceOccurrenceStages + ":" + models.ActionRead,
	models.ResourceOccurrenceStages + ":" + models.ActionWrite,
	models.ResourceOccurrenceStages + ":" + models.ActionDelete,
}

// BackfillOccurrencePermissions concede as permissões do CRM aos papéis que já
// têm a capacidade equivalente por chat e por configurações gerais.
//
// Existe porque FixSystemRolePermissions pula qualquer papel que já tenha
// permissões, para não desfazer customizações — o que significa que uma
// permissão nova jamais chega a uma instalação existente por aquele caminho.
//
// É PURAMENTE ADITIVO: nunca revoga nada. Essa é a propriedade que torna o
// rollback seguro, porque a versão anterior autoriza por chat:* e ele
// permanece intacto.
//
// Idempotência é pela forma do dado, como BackfillChatbotFlowGraph: uma
// organização que já tenha qualquer papel com qualquer permissão occurrences%
// é pulada inteira, então isto roda uma vez por organização. Essa guarda de
// "já migrada" vive inteira dentro do INSERT (ver nota no SQL abaixo), não
// como uma lista de ids calculada à parte — isso evita tanto a divergência
// entre a checagem e a gravação quanto o limite de 65535 parâmetros do
// Postgres por statement, que uma lista de ids materializada esbarraria
// perto de ~65 mil organizações.
func BackfillOccurrencePermissions(db *gorm.DB, lo logf.Logger) error {
	// As linhas de permissão são criadas por SeedPermissionsAndRoles. Se ainda
	// não existem, não há o que ligar e este boot não faz nada. Isso é
	// silencioso por padrão — sem este Warn, um boot que rodou antes do seed
	// fica indistinguível no log de um boot que migrou tudo, e o primeiro
	// sintoma vira 403 para todo mundo no CRM.
	//
	// A checagem compara contra occurrencePermissionKeys chave por chave (não
	// só a contagem): um sexto permission sob "occurrences" somado a um dos
	// cinco renomeado ainda bateria len()==5 e passaria a guarda com o
	// conjunto errado.
	var seededRows []string
	if err := db.Model(&models.Permission{}).
		Where("resource IN ?", []string{models.ResourceOccurrences, models.ResourceOccurrenceStages}).
		Pluck("resource || ':' || action", &seededRows).Error; err != nil {
		return fmt.Errorf("failed to count occurrence permissions: %w", err)
	}
	seeded := make(map[string]bool, len(seededRows))
	for _, k := range seededRows {
		seeded[k] = true
	}
	for _, key := range occurrencePermissionKeys {
		if !seeded[key] {
			lo.Warn("Occurrence permissions backfill: occurrence permissions not seeded yet, did nothing",
				"resources", []string{models.ResourceOccurrences, models.ResourceOccurrenceStages})
			return nil
		}
	}

	// Só para o log: quantas organizações ainda não têm nenhum papel com
	// permissão occurrences%. A checagem que realmente decide o que grava é
	// a NOT EXISTS correlacionada dentro do INSERT logo abaixo — esta conta
	// não precisa (e não deve) ser reusada como lista de parâmetros.
	//
	// Sem filtro de o.deleted_at aqui de propósito: o INSERT abaixo também
	// não junta organizations, então processa papéis vivos dentro de
	// organizações soft-deleted. Contar só as vivas aqui faria esta conta
	// divergir do que o INSERT realmente decide gravar.
	var pendingOrgs int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM organizations o
		WHERE NOT EXISTS (
			SELECT 1
			FROM custom_roles r
			JOIN role_permissions rp ON rp.custom_role_id = r.id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE r.organization_id = o.id
			  AND r.deleted_at IS NULL
			  AND p.resource LIKE 'occurrences%'
		  )`).Scan(&pendingOrgs).Error; err != nil {
		return fmt.Errorf("failed to count organisations pending the occurrence backfill: %w", err)
	}
	if pendingOrgs == 0 {
		lo.Info("Occurrence permissions backfill: nothing pending, all organisations already migrated")
		return nil
	}

	// Todas as regras de concessão viram UM único INSERT (a tabela g abaixo
	// é o occurrenceGrants em forma de VALUES), não um loop com um INSERT
	// por regra. Isso importa: dentro de um único statement o Postgres lê
	// role_permissions a partir de um snapshot tirado no início do
	// statement, então a guarda "organização ainda não migrada" não vê as
	// próprias linhas que este mesmo INSERT está gravando. Um loop de N
	// statements não teria essa propriedade — o primeiro INSERT (chat:read
	// -> occurrences:read) gravaria a permissão que o segundo INSERT
	// (chat:write -> occurrences:write) usa como sinal de "já migrada", e a
	// organização pareceria migrada a partir do segundo statement em
	// diante, perdendo as concessões seguintes no mesmo boot.
	placeholders := make([]string, len(occurrenceGrants))
	args := make([]any, 0, len(occurrenceGrants)*4)
	for i, g := range occurrenceGrants {
		placeholders[i] = "(?,?,?,?)"
		args = append(args, g.fromResource, g.fromAction, g.toResource, g.toAction)
	}

	query := fmt.Sprintf(`
		INSERT INTO role_permissions (custom_role_id, permission_id)
		SELECT r.id, target.id
		FROM custom_roles r
		JOIN role_permissions rp ON rp.custom_role_id = r.id
		JOIN permissions src ON src.id = rp.permission_id
		JOIN (VALUES %s) AS g(from_resource, from_action, to_resource, to_action)
		  ON src.resource = g.from_resource AND src.action = g.from_action
		JOIN permissions target
		  ON target.resource = g.to_resource AND target.action = g.to_action
		WHERE r.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM role_permissions existing
			WHERE existing.custom_role_id = r.id
			  AND existing.permission_id = target.id
		  )
		  -- A guarda "organização ainda não migrada", correlacionada
		  -- diretamente em r.organization_id em vez de uma lista de ids
		  -- vinda de fora: um só lugar decide isso, não um SELECT separado
		  -- reexpandido como parâmetros do INSERT (o que esbarraria no
		  -- limite de 65535 parâmetros do Postgres por statement perto de
		  -- ~65 mil organizações).
		  AND NOT EXISTS (
			SELECT 1
			FROM custom_roles r2
			JOIN role_permissions rp2 ON rp2.custom_role_id = r2.id
			JOIN permissions p2 ON p2.id = rp2.permission_id
			WHERE r2.organization_id = r.organization_id
			  AND r2.deleted_at IS NULL
			  AND p2.resource LIKE 'occurrences%%'
		  )
		ON CONFLICT (custom_role_id, permission_id) DO NOTHING`,
		strings.Join(placeholders, ","),
	)

	res := db.Exec(query, args...)
	if res.Error != nil {
		return fmt.Errorf("failed to grant occurrence permissions: %w", res.Error)
	}

	lo.Info("Occurrence permissions backfill complete",
		"organisations_processed", pendingOrgs, "links_granted", res.RowsAffected)
	return nil
}

// BackfillContactNamePermission concede contacts.name:write aos papéis que
// já renomeiam contatos hoje e aos que atendem conversas.
//
// ATENÇÃO — esta regra difere DE PROPÓSITO da de BackfillOccurrencePermissions.
// Lá a regra era equivalência exata: ninguém ganhava capacidade nova. Aqui a
// segunda origem, chat:write, é uma AMPLIAÇÃO deliberada — o produto pediu que
// quem atende possa corrigir o nome do contato sem depender de um gestor.
//
// Puramente aditivo, como o outro: nunca revoga nada, então um rollback
// continua funcionando com as permissões antigas intactas.
func BackfillContactNamePermission(db *gorm.DB, lo logf.Logger) error {
	var seeded int64
	if err := db.Model(&models.Permission{}).
		Where("resource = ? AND action = ?", models.ResourceContactName, models.ActionWrite).
		Count(&seeded).Error; err != nil {
		return fmt.Errorf("failed to count the contact name permission: %w", err)
	}
	if seeded == 0 {
		lo.Warn("contacts.name permission not seeded yet, did nothing")
		return nil
	}

	// SELECT DISTINCT importa: um papel que tenha as duas origens (contacts:write
	// e chat:write) casaria duas vezes contra o CROSS JOIN e tentaria inserir o
	// mesmo vínculo em duplicidade dentro da mesma sentença, que o ON CONFLICT
	// não cobre.
	res := db.Exec(`
		INSERT INTO role_permissions (custom_role_id, permission_id)
		SELECT DISTINCT r.id, target.id
		FROM custom_roles r
		JOIN role_permissions rp ON rp.custom_role_id = r.id
		JOIN permissions src ON src.id = rp.permission_id
		CROSS JOIN permissions target
		WHERE r.deleted_at IS NULL
		  AND target.resource = ? AND target.action = ?
		  AND (
		    (src.resource = ? AND src.action = ?)
		    OR (src.resource = ? AND src.action = ?)
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM custom_roles r2
		    JOIN role_permissions rp2 ON rp2.custom_role_id = r2.id
		    JOIN permissions p2 ON p2.id = rp2.permission_id
		    WHERE r2.organization_id = r.organization_id
		      AND r2.deleted_at IS NULL
		      AND p2.resource = ?
		  )
		ON CONFLICT DO NOTHING`,
		models.ResourceContactName, models.ActionWrite,
		models.ResourceContacts, models.ActionWrite,
		models.ResourceChat, models.ActionWrite,
		models.ResourceContactName,
	)
	if res.Error != nil {
		return fmt.Errorf("failed to grant the contact name permission: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		lo.Info("contact name backfill: nothing pending")
		return nil
	}
	lo.Info("contact name backfill complete", "links_granted", res.RowsAffected)
	return nil
}
