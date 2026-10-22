package migrations

import "testing"

func TestConsolidatedModelBudgetRepairPreservesExplicitBindings(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"migrated", "revised", "audited", "different_model"} {
		t.Run(scenario, func(t *testing.T) {
			if _, err := pool.Exec(ctx, `TRUNCATE public.model_roles; DELETE FROM public.configuration_revisions WHERE resource_key LIKE 'binding:%'; DELETE FROM public.owner_surface_audits WHERE action='provider_binding_update';
    INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,max_input_tokens) VALUES('action_realization','endpoint','model',57344),('generic_llm','endpoint','model',49152);
    UPDATE public.alembic_version SET version_num='0097_owner_schedules';`); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "revised":
				if _, err := pool.Exec(ctx, `UPDATE public.configuration_revisions SET revision=2 WHERE resource_key='binding:generic_llm'`); err != nil {
					t.Fatal(err)
				}
			case "audited":
				if _, err := pool.Exec(ctx, `INSERT INTO public.owner_surface_audits(id,owner_actor_id,fluctlight_id,identity,actor,action,reason,record_id) VALUES('budget-audit','owner','','owner_edit','owner','provider_binding_update','修改配置资源：["binding:generic_llm"]','config')`); err != nil {
					t.Fatal(err)
				}
			case "different_model":
				if _, err := pool.Exec(ctx, `UPDATE public.model_roles SET model_id='other' WHERE role='generic_llm'`); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				if err := New(pool).Apply(ctx); err != nil {
					t.Fatal(err)
				}
			}
			var limit int
			if err := pool.QueryRow(ctx, `SELECT max_input_tokens FROM public.model_roles WHERE role='generic_llm'`).Scan(&limit); err != nil {
				t.Fatal(err)
			}
			expected := 49152
			if scenario == "migrated" {
				expected = 57344
			}
			if limit != expected {
				t.Fatalf("%s: limit=%d expected=%d", scenario, limit, expected)
			}
		})
	}
}

func TestConsolidationCopiesLegacyBudgetAndPreservesCustomTriple(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE public.model_roles; DELETE FROM public.configuration_revisions WHERE resource_key LIKE 'binding:%';
 INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,context_window_tokens,max_input_tokens,prompt_budget_policy_version) VALUES('action_realization','endpoint','model',131072,100000,'prompt-budget.v2'); UPDATE public.alembic_version SET version_num='0097_owner_schedules';`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var window, input int
	var policy string
	if err := pool.QueryRow(ctx, `SELECT context_window_tokens,max_input_tokens,prompt_budget_policy_version FROM public.model_roles WHERE role='generic_llm'`).Scan(&window, &input, &policy); err != nil {
		t.Fatal(err)
	}
	if window != 131072 || input != 100000 || policy != "prompt-budget.v2" {
		t.Fatal(window, input, policy)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.model_roles SET context_window_tokens=65536,max_input_tokens=40000,prompt_budget_policy_version='prompt-budget.v1' WHERE role='generic_llm'; UPDATE public.configuration_revisions SET revision=1 WHERE resource_key='binding:generic_llm'; UPDATE public.alembic_version SET version_num='0097_owner_schedules';`); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT max_input_tokens FROM public.model_roles WHERE role='generic_llm'`).Scan(&input); err != nil || input != 40000 {
		t.Fatal(input, err)
	}
}
